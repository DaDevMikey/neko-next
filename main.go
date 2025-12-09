package main

import (
	"bytes"
	"embed"
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
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"crg.eti.br/go/config"
	_ "crg.eti.br/go/config/ini"
)

// --- Windows API for Global Mouse Position ---
var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type POINT struct {
	X, Y int32
}

func getGlobalMousePos() (int, int) {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

// ---------------------------------------------

type neko struct {
	waiting    bool
	x          float64
	y          float64
	distance   int
	count      int
	min        int
	max        int
	state      int
	sprite     string
	lastSprite string
	img        *ebiten.Image
}

type Config struct {
	Speed            float64 `cfg:"speed" cfgDefault:"2.0"`
	Scale            float64 `cfg:"scale" cfgDefault:"2.0"`
	Alpha            float64 `cfg:"alpha" cfgDefault:"1.0"` // Transparency
	Quiet            bool    `cfg:"quiet" cfgDefault:"false"`
	MousePassthrough bool    `cfg:"mousepassthrough" cfgDefault:"true"`
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

	monitorWidth, monitorHeight = ebiten.Monitor().Size()
	cfg                         = &Config{}
	currentplayer               *audio.Player = nil

	// Tray Menu Items
	mClickThrough *systray.MenuItem
	mSleep        *systray.MenuItem
	mTeleport     *systray.MenuItem // New feature!
)

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
	// Force "Always on Top" behavior
	ebiten.SetWindowFloating(true)

	m.count++
	if m.state == 10 && m.count == m.min {
		playSound(mSound["idle3"])
	}

	// 1. Get Physical Mouse Position (Global)
	mx, my := getGlobalMousePos()

	// 2. Fix for DPI Scaling (The "Offset" Bug Fix)
	// We divide the physical coordinates by the scale factor to get logical coordinates
	scaleFactor := ebiten.Monitor().DeviceScaleFactor()
	mx = int(float64(mx) / scaleFactor)
	my = int(float64(my) / scaleFactor)

	// Calculate target position (center of the cat)
	// Current window position
	wx, wy := int(m.x), int(m.y)
	
	// Target is where the mouse is, centered relative to the cat's size
	trgX := mx - (int(float64(width)*cfg.Scale) / 2)
	trgY := my - (int(float64(height)*cfg.Scale) / 2)

	dx := trgX - wx
	dy := trgY - wy

	m.distance = int(math.Sqrt(float64(dx*dx + dy*dy)))

	// Check if Neko is sleeping or waiting
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

	// Keep within monitor bounds
	// Note: We use raw logical pixels here, assuming single monitor for simplicity
	// (Multi-monitor clamping is complex, so we just clamp to 0,0 min)
	m.x = math.Max(0, m.x) 
	m.y = math.Max(0, m.y)
	
	ebiten.SetWindowPosition(int(m.x), int(m.y))

	// Determine sprite direction
	m.catchCursor(dx, dy)
	return nil
}

func (m *neko) stayIdle() {
	switch m.state {
	case 0:
		m.state = 1
		fallthrough
	case 1, 2, 3:
		m.sprite = "awake"
	case 4, 5, 6:
		m.sprite = "scratch"
	case 7, 8, 9:
		m.sprite = "wash"
	case 10, 11, 12:
		m.min = 32
		m.max = 64
		m.sprite = "yawn"
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

	if m.lastSprite == sprite {
		return
	}

	m.lastSprite = sprite
	screen.Clear()
	
	// Apply Transparency (Ghost Mode)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(cfg.Alpha))
	
	screen.DrawImage(m.img, op)
}

// --- TRAY MENU LOGIC ---

func onReady() {
	iconData, _ := f.ReadFile("assets/awake.png")
	systray.SetIcon(iconData)
	systray.SetTitle("Neko")
	systray.SetTooltip("Neko - The Desktop Cat")

	mSleep = systray.AddMenuItemCheckbox("Sleep", "Put Neko to sleep", false)
	mTeleport = systray.AddMenuItem("Teleport to Mouse", "Bring Neko to you immediately")
	
	systray.AddSeparator()

	mClickThrough = systray.AddMenuItemCheckbox("Click Through", "Let mouse clicks pass through Neko", cfg.MousePassthrough)
	mSoundMenu := systray.AddMenuItemCheckbox("Sound", "Enable/Disable sound", !cfg.Quiet)
	
	// Speed Menu
	mSpeed := systray.AddMenuItem("Speed", "Change running speed")
	mSpeedSlow := mSpeed.AddSubMenuItem("Slow", "Slow speed")
	mSpeedNormal := mSpeed.AddSubMenuItem("Normal", "Normal speed")
	mSpeedFast := mSpeed.AddSubMenuItem("Zoomies!", "Fast speed")

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
	mQuit := systray.AddMenuItem("Exit", "Bye bye Neko")

	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
			
			// Teleport Logic
			case <-mTeleport.ClickedCh:
				mx, my := getGlobalMousePos()
				s := ebiten.Monitor().DeviceScaleFactor()
				nekoInstance.x = float64(mx) / s
				nekoInstance.y = float64(my) / s
				ebiten.SetWindowPosition(int(nekoInstance.x), int(nekoInstance.y))

			case <-mSleep.ClickedCh:
				if mSleep.Checked() {
					mSleep.Uncheck()
					nekoInstance.waiting = false
				} else {
					mSleep.Check()
					nekoInstance.waiting = true
					nekoInstance.state = 13 // Force sleep animation
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

func onExit() {
	os.Exit(0)
}

var nekoInstance *neko

func main() {
	config.PrefixEnv = "NEKO"
	config.File = "neko.ini"
	config.Parse(cfg)

	// Default to Click Through = TRUE
	cfg.MousePassthrough = true 

	go func() {
		systray.Run(onReady, onExit)
	}()

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
				log.Fatal(err)
			}
			mSprite[name] = ebiten.NewImageFromImage(img)
		case ".wav":
			stream, err := wav.DecodeWithSampleRate(44100, bytes.NewReader(data))
			if err != nil {
				log.Fatal(err)
			}
			data, err := io.ReadAll(stream)
			if err != nil {
				log.Fatal(err)
			}
			mSound[name] = data
		}
	}

	audio.NewContext(44100)
	audio.CurrentContext().NewPlayerFromBytes([]byte{}).Play()

	nekoInstance = &neko{
		x:   float64(monitorWidth / 2),
		y:   float64(monitorHeight / 2),
		min: 8,
		max: 16,
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

	err := ebiten.RunGameWithOptions(nekoInstance, &ebiten.RunGameOptions{
		InitUnfocused:     true,
		ScreenTransparent: true,
		SkipTaskbar:       true,
		X11ClassName:      "Neko",
		X11InstanceName:   "Neko",
	})
	if err != nil {
		log.Fatal(err)
	}
}
