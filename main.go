package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	_ "crg.eti.br/go/config/ini"
	"github.com/getlantern/systray"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"golang.org/x/sys/windows/registry"
)

// --- Windows API for Global Mouse Position ---
var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

type POINT struct {
	X, Y int32
}

func getGlobalMousePos() (int, int) {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

const (
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_XVIRTUALSCREEN  = 76 // Left of virtual screen (can be negative)
	SM_YVIRTUALSCREEN  = 77 // Top of virtual screen (can be negative)
	SM_CXVIRTUALSCREEN = 78 // Width of virtual screen
	SM_CYVIRTUALSCREEN = 79 // Height of virtual screen
)

func getPrimaryScreenRect() (w, h int) {
	r1, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	r2, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSCREEN))
	return int(r1), int(r2)
}

// getVirtualScreenBounds returns the bounds of the entire virtual screen
// (all monitors combined). Coordinates can be negative for monitors
// positioned to the left or above the primary monitor.
func getVirtualScreenBounds() (x, y, w, h int) {
	x1, _, _ := procGetSystemMetrics.Call(uintptr(SM_XVIRTUALSCREEN))
	y1, _, _ := procGetSystemMetrics.Call(uintptr(SM_YVIRTUALSCREEN))
	w1, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXVIRTUALSCREEN))
	h1, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYVIRTUALSCREEN))
	return int(int32(x1)), int(int32(y1)), int(w1), int(h1)
}

// ---------------------------------------------

type neko struct {
	waiting             bool
	manualSleep         bool // Track if sleep was triggered manually (vs StayOnPrimary)
	hidden              bool // Temporarily hide the cat
	x                   float64
	y                   float64
	distance            int
	count               int
	min                 int
	max                 int
	state               int
	sprite              string
	lastSprite          string
	lastSleepToggleTime time.Time // For debouncing sleep shortcut
	img                 *ebiten.Image

	// cmdChan is used to send commands from the systray to the game loop safely.
	cmdChan chan func(*neko)
}

type Config struct {
	Speed            float64 `cfg:"speed" cfgDefault:"2.0"`
	Scale            float64 `cfg:"scale" cfgDefault:"2.0"`
	Alpha            float64 `cfg:"alpha" cfgDefault:"1.0"` // Transparency
	Quiet            bool    `cfg:"quiet" cfgDefault:"false"`
	MousePassthrough bool    `cfg:"mousepassthrough" cfgDefault:"true"`
	StayOnPrimary    bool    `cfg:"stayonprimary" cfgDefault:"true"`
	RunOnStartup     bool    `cfg:"runonstartup" cfgDefault:"false"`
}

const (
	width  = 32
	height = 32
)

var (
	mSprite map[string]*ebiten.Image
	mSound  map[string][]byte

	//go:embed assets/*
	f embed.FS

	monitorWidth, monitorHeight               = ebiten.Monitor().Size()
	cfg                                       = &Config{}
	currentplayer               *audio.Player = nil

	// Tray Menu Items
	mClickThrough  *systray.MenuItem
	mSleep         *systray.MenuItem
	mHide          *systray.MenuItem // Temporarily hide the cat
	mTeleport      *systray.MenuItem // New feature!
	mStayOnPrimary *systray.MenuItem
	mRunOnStartup  *systray.MenuItem
)

// sendCmd safely sends a function to be executed on the main game thread.
func (m *neko) sendCmd(cmd func(*neko)) {
	m.cmdChan <- cmd
}

func (m *neko) Layout(outsideWidth, outsideHeight int) (int, int) {
	return width, height
}

func playSound(sound []byte) {
	if cfg.Quiet {
		return
	}
	if currentplayer != nil && currentplayer.IsPlaying() {
		currentplayer.Close()
	}
	currentplayer = audio.CurrentContext().NewPlayerFromBytes(sound)
	currentplayer.SetVolume(.3)
	currentplayer.Play()
}

func (m *neko) Update() error {
	// Process all pending commands from the systray channel.
	// This is the only safe way to modify neko's state from another goroutine.
processCmds:
	for {
		select {
		case cmd := <-m.cmdChan:
			cmd(m)
		default:
			break processCmds
		}
	}

	ebiten.SetWindowFloating(true) // Ensure window stays on top.

	m.count++
	if m.state == 10 && m.count == m.min {
		playSound(mSound["idle3"])
	}

	// 1. Get Physical Mouse Position (Global)
	mx, my := getGlobalMousePos()

	// 2. Fix for DPI Scaling (The "Offset" Bug Fix)
	// We divide the physical mouse coordinates by the monitor's scale factor
	// to get the correct target coordinates in the logical space Ebiten uses.
	scaleFactor := ebiten.Monitor().DeviceScaleFactor()
	targetX := float64(mx)
	targetY := float64(my)

	// The cat's position (m.x, m.y) is in physical pixels.
	// The target is the mouse position, adjusted so the cat's center is on the cursor.
	winWidth, winHeight := ebiten.WindowSize()
	trgX := targetX - (float64(winWidth) * scaleFactor / 2)
	trgY := targetY - (float64(winHeight) * scaleFactor / 2)

	dx := int(trgX - m.x)
	dy := int(trgY - m.y)
	m.distance = int(math.Sqrt(float64(dx*dx + dy*dy)))

	// Stay on Primary Check - only affects automatic sleep, not manual sleep
	if cfg.StayOnPrimary && !m.manualSleep {
		pw, ph := getPrimaryScreenRect()
		if mx < 0 || mx > pw || my < 0 || my > ph {
			// Mouse is off primary monitor - sleep if not already sleeping
			if !m.waiting {
				m.waiting = true
				m.state = 13 // Force sleep animation
			}
			m.stayIdle()
			return nil
		} else {
			// Mouse is on primary monitor - wake up only if sleeping due to StayOnPrimary
			// (not manual sleep)
			if m.waiting && !m.manualSleep {
				m.waiting = false
				m.state = 0 // Reset state to allow normal animation
				// Also uncheck the Sleep menu item since we're waking up
				if mSleep != nil && mSleep.Checked() {
					mSleep.Uncheck()
				}
			}
		}
	}

	// Check if Neko is sleeping or waiting (for manual sleep toggle)
	if m.waiting {
		m.stayIdle()
		return nil
	}

	// If close enough to the mouse, stop moving
	if m.distance < int(float64(width)*cfg.Scale) {
		m.stayIdle()
		return nil
	}

	if m.state >= 13 {
		playSound(mSound["awake"])
	}

	// Move towards the mouse
	angle := math.Atan2(float64(dy), float64(dx))

	moveX := math.Cos(angle) * cfg.Speed
	moveY := math.Sin(angle) * cfg.Speed

	m.x += moveX
	m.y += moveY

	// Keep within virtual screen bounds (all monitors)
	// This properly handles multi-monitor setups including vertical arrangements
	// and monitors with negative coordinates (left/above primary)
	vx, vy, vw, vh := getVirtualScreenBounds()
	catWidth := float64(winWidth) * scaleFactor
	catHeight := float64(winHeight) * scaleFactor

	m.x = math.Max(float64(vx), math.Min(m.x, float64(vx+vw)-catWidth))
	m.y = math.Max(float64(vy), math.Min(m.y, float64(vy+vh)-catHeight))

	// SetWindowPosition expects logical pixels, so we convert back.
	logicalX := int(m.x / scaleFactor)
	logicalY := int(m.y / scaleFactor)
	ebiten.SetWindowPosition(logicalX, logicalY)

	// Determine sprite direction
	m.catchCursor(dx, dy)
	return nil
}

func (m *neko) stayIdle() {
	switch m.state {
	case 0:
		m.state = 1
		fallthrough
	case 1:
		m.sprite = "awake"
	case 2, 3:
		m.sprite = "alert"
	case 4, 5:
		m.sprite = "scratch"
	case 6, 7:
		m.sprite = "wash"
	case 8, 9:
		m.min = 32
		m.max = 64
		m.sprite = "yawn"
	case 10, 11, 12:
		// Additional idle time before sleeping
		m.sprite = "awake"
	default:
		m.sprite = "sleep"
	}
}

func (m *neko) catchCursor(dx, dy int) {
	m.state = 0
	m.min = 8
	m.max = 16

	r := math.Atan2(float64(dy), float64(dx))
	a := math.Mod((r/math.Pi*180)+360, 360)

	switch {
	case a <= 292.5 && a > 247.5:
		m.sprite = "up"
	case a <= 337.5 && a > 292.5:
		m.sprite = "upright"
	case a <= 22.5 || a > 337.5:
		m.sprite = "right"
	case a <= 67.5 && a > 22.5:
		m.sprite = "downright"
	case a <= 112.5 && a > 67.5:
		m.sprite = "down"
	case a <= 157.5 && a > 112.5:
		m.sprite = "downleft"
	case a <= 202.5 && a > 157.5:
		m.sprite = "left"
	case a <= 247.5 && a > 202.5:
		m.sprite = "upleft"
	}
}

func (m *neko) Draw(screen *ebiten.Image) {
	// Always clear the screen first
	screen.Clear()

	// If hidden, don't draw anything
	if m.hidden {
		return
	}

	var sprite string
	switch {
	case m.sprite == "awake":
		sprite = m.sprite
	case m.count < m.min:
		sprite = m.sprite + "1"
	default:
		sprite = m.sprite + "2"
	}

	m.img = mSprite[sprite]
	if m.img == nil {
		log.Printf("Warning: Missing sprite image for '%s'. Using 'awake'.", sprite)
		m.img = mSprite["awake"] // Fallback to a known good sprite
		// If even "awake" is missing, we have a bigger problem, but at least we won't crash here.
	}

	if m.count > m.max {
		m.count = 0
		if m.state > 0 {
			m.state++
			switch m.state {
			case 13:
				playSound(mSound["sleep"])
			}
		}
	}

	// Apply Transparency (Ghost Mode)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(cfg.Alpha))

	screen.DrawImage(m.img, op)
}

// --- TRAY MENU LOGIC ---

func onReady() {
	iconData, err := f.ReadFile("assets/icon.ico")
	if err != nil {
		log.Printf("Warning: Failed to load system tray icon: %v", err)
	} else {
		systray.SetIcon(iconData)
	}
	systray.SetTitle("Neko Next")
	systray.SetTooltip("Neko Next- The Upgraded Desktop Cat")

	mSleep = systray.AddMenuItemCheckbox("Sleep", "Put Neko to sleep", false)
	mHide = systray.AddMenuItemCheckbox("Hide", "Temporarily hide Neko (still running)", false)
	mTeleport = systray.AddMenuItem("Teleport to Mouse", "Bring Neko to you immediately")

	systray.AddSeparator()
	mStayOnPrimary = systray.AddMenuItemCheckbox("Stay on Primary", "Put Neko to sleep when leaving the main screen", cfg.StayOnPrimary)

	mClickThrough = systray.AddMenuItemCheckbox("Click Through", "Let mouse clicks pass through Neko", cfg.MousePassthrough)
	mSoundMenu := systray.AddMenuItemCheckbox("Sound", "Enable/Disable sound", !cfg.Quiet)

	// Speed Menu
	mSpeed := systray.AddMenuItem("Speed", "Change running speed")
	mSpeedSlow := mSpeed.AddSubMenuItem("Slow", "Slow speed")
	mSpeedNormal := mSpeed.AddSubMenuItem("Normal", "Normal speed")
	mSpeedFast := mSpeed.AddSubMenuItem("Zoomies!", "Fast speed")
	mSpeedLudicrous := mSpeed.AddSubMenuItem("Ludicrous!", "Ludicrous speed")

	// Size Menu
	mScale := systray.AddMenuItem("Size", "Change size")
	mScaleSmall := mScale.AddSubMenuItem("Small (1x)", "Small")
	mScaleNormal := mScale.AddSubMenuItem("Normal (2x)", "Normal")
	mScaleBig := mScale.AddSubMenuItem("Big (3x)", "Big")

	// Transparency Menu (New!)
	mAlpha := systray.AddMenuItem("Opacity", "Change transparency")
	mAlphaSolid := mAlpha.AddSubMenuItem("Solid (100%)", "Fully visible")
	mAlphaGhost := mAlpha.AddSubMenuItem("Ghost (50%)", "Semi-transparent")
	mAlphaInvisible := mAlpha.AddSubMenuItem("Ninja (20%)", "Almost invisible")

	systray.AddSeparator()
	mRunOnStartup = systray.AddMenuItemCheckbox("Run on Startup", "Start automatically with Windows", cfg.RunOnStartup)
	mRestart := systray.AddMenuItem("Restart", "Restart Neko")
	mQuit := systray.AddMenuItem("Exit", "Bye bye Neko")

	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				// Save settings before quitting
				saveSettings()
				systray.Quit()

			case <-mRestart.ClickedCh:
				// Save settings and restart the application
				saveSettings()
				restart()

			// Teleport Logic
			case <-mTeleport.ClickedCh:
				nekoInstance.sendCmd(func(n *neko) {
					mx, my := getGlobalMousePos()
					// Set position in physical pixels
					n.x = float64(mx)
					n.y = float64(my)
				})

			case <-mSleep.ClickedCh:
				if mSleep.Checked() {
					mSleep.Uncheck()
					nekoInstance.sendCmd(func(n *neko) {
						n.waiting = false
						n.manualSleep = false
					})
				} else {
					mSleep.Check()
					nekoInstance.sendCmd(func(n *neko) {
						n.waiting = true
						n.manualSleep = true
						n.state = 13 // Force sleep animation
					})
				}
			case <-mHide.ClickedCh:
				if mHide.Checked() {
					mHide.Uncheck()
					nekoInstance.sendCmd(func(n *neko) { n.hidden = false })
				} else {
					mHide.Check()
					nekoInstance.sendCmd(func(n *neko) { n.hidden = true })
				}
			case <-mStayOnPrimary.ClickedCh:
				if mStayOnPrimary.Checked() {
					mStayOnPrimary.Uncheck()
					cfg.StayOnPrimary = false
				} else {
					mStayOnPrimary.Check()
					cfg.StayOnPrimary = true
				}
			case <-mRunOnStartup.ClickedCh:
				if mRunOnStartup.Checked() {
					setStartup(false)
				} else {
					setStartup(true)
				}
			case <-mClickThrough.ClickedCh:
				if mClickThrough.Checked() {
					mClickThrough.Uncheck()
					cfg.MousePassthrough = false
				} else {
					mClickThrough.Check()
					cfg.MousePassthrough = true
				}
				ebiten.SetWindowMousePassthrough(cfg.MousePassthrough)
			case <-mSoundMenu.ClickedCh:
				if mSoundMenu.Checked() {
					mSoundMenu.Uncheck()
					cfg.Quiet = true
				} else {
					mSoundMenu.Check()
					cfg.Quiet = false
				}

			// Speed
			case <-mSpeedSlow.ClickedCh:
				cfg.Speed = 1.0
			case <-mSpeedNormal.ClickedCh:
				cfg.Speed = 2.0
			case <-mSpeedFast.ClickedCh:
				cfg.Speed = 4.0
			case <-mSpeedLudicrous.ClickedCh:
				cfg.Speed = 8.0

			// Scale
			case <-mScaleSmall.ClickedCh:
				updateScale(1.0)
			case <-mScaleNormal.ClickedCh:
				updateScale(2.0)
			case <-mScaleBig.ClickedCh:
				updateScale(3.0)

			// Opacity
			case <-mAlphaSolid.ClickedCh:
				cfg.Alpha = 1.0
			case <-mAlphaGhost.ClickedCh:
				cfg.Alpha = 0.5
			case <-mAlphaInvisible.ClickedCh:
				cfg.Alpha = 0.2
			}
		}
	}()
}

func updateScale(s float64) {
	cfg.Scale = s
	ebiten.SetWindowSize(int(float64(width)*s), int(float64(height)*s))
}

func saveSettings() {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("Error marshaling settings: %v", err)
		return
	}
	err = os.WriteFile("neko_settings.json", data, 0666)
	if err != nil {
		log.Printf("Error saving settings to neko_settings.json: %v", err)
	}
}

func restart() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path for restart: %v", err)
		return
	}

	log.Println("Restarting Neko...")

	// Start a new instance of the application
	cmd := &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	procAttr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   cmd,
	}

	_, err = os.StartProcess(exePath, []string{exePath}, procAttr)
	if err != nil {
		log.Printf("Error starting new process: %v", err)
		return
	}

	// Exit the current instance
	systray.Quit()
}

func setStartup(enabled bool) {
	cfg.RunOnStartup = enabled
	if mRunOnStartup != nil {
		if enabled {
			mRunOnStartup.Check()
		} else {
			mRunOnStartup.Uncheck()
		}
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		log.Printf("Error opening registry key: %v", err)
		return
	}
	defer key.Close()

	if enabled {
		exePath, _ := os.Executable()
		err = key.SetStringValue("Neko", `"`+exePath+`"`)
	} else {
		err = key.DeleteValue("Neko")
	}
	log.Printf("Run on startup set to %v. Error: %v", enabled, err)
}

func onExit() {
	os.Exit(0)
}

var nekoInstance *neko

func main() {
	// Setup file-based logging to capture errors.
	logFile, err := os.OpenFile("neko.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.Println("Starting Neko...")

	// Load settings from JSON file.
	data, err := os.ReadFile("neko_settings.json")
	if err == nil {
		err = json.Unmarshal(data, cfg)
		if err != nil {
			log.Printf("Error unmarshaling settings: %v", err)
		}
	} else {
		log.Println("No neko_settings.json found, using defaults.")
	}

	// Set startup registry key based on loaded config
	setStartup(cfg.RunOnStartup)

	// Systray must run in a separate goroutine.
	go func() {
		systray.Run(onReady, onExit)
	}()
	log.Println("System tray routine started.")

	mSprite = make(map[string]*ebiten.Image)
	mSound = make(map[string][]byte)

	a, _ := fs.ReadDir(f, "assets")
	for _, v := range a {
		data, _ := f.ReadFile("assets/" + v.Name())
		name := strings.TrimSuffix(v.Name(), filepath.Ext(v.Name()))
		ext := filepath.Ext(v.Name())

		switch ext {
		case ".png":
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				log.Printf("Error decoding png %s: %v", name, err)
				continue
			}
			mSprite[name] = ebiten.NewImageFromImage(img)
		case ".wav":
			stream, err := wav.DecodeWithSampleRate(44100, bytes.NewReader(data))
			if err != nil {
				log.Printf("Error decoding wav %s: %v", name, err)
				continue
			}
			data, err := io.ReadAll(stream)
			if err != nil {
				log.Printf("Error reading wav stream %s: %v", name, err)
				continue
			}
			mSound[name] = data
		}
	}
	log.Println("Assets loaded.")

	audio.NewContext(44100)

	nekoInstance = &neko{
		x:       float64(monitorWidth / 2),
		y:       float64(monitorHeight / 2),
		min:     8,
		max:     16,
		cmdChan: make(chan func(*neko), 10), // Buffered channel for commands
	}

	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)

	// Set click-through state at startup
	ebiten.SetWindowMousePassthrough(cfg.MousePassthrough)
	ebiten.SetWindowSize(int(float64(width)*cfg.Scale), int(float64(height)*cfg.Scale))
	ebiten.SetWindowTitle("Neko")

	log.Println("Starting Ebiten game loop...")
	if err := ebiten.RunGameWithOptions(nekoInstance, &ebiten.RunGameOptions{
		InitUnfocused:     true,
		ScreenTransparent: true,
		SkipTaskbar:       true,
		X11ClassName:      "Neko",
		X11InstanceName:   "Neko",
	}); err != nil {
		log.Fatal(err)
	}
}
