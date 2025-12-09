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

	"github.com/getlantern/systray"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"crg.eti.br/go/config"
	_ "crg.eti.br/go/config/ini"
)

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
	Speed            float64 `cfg:"speed" cfgDefault:"2.0" cfgHelper:"The speed of the cat."`
	Scale            float64 `cfg:"scale" cfgDefault:"2.0" cfgHelper:"The scale of the cat."`
	Quiet            bool    `cfg:"quiet" cfgDefault:"false" cfgHelper:"Disable sound."`
	MousePassthrough bool    `cfg:"mousepassthrough" cfgDefault:"false" cfgHelper:"Enable mouse passthrough."`
}

const (
	width  = 32
	height = 32
)

var (
	loaded  = false
	mSprite map[string]*ebiten.Image
	mSound  map[string][]byte

	//go:embed assets/*
	f embed.FS

	monitorWidth, monitorHeight = ebiten.Monitor().Size()

	cfg = &Config{}

	currentplayer *audio.Player = nil
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
	m.count++
	if m.state == 10 && m.count == m.min {
		playSound(mSound["idle3"])
	}
	
	m.x = max(0, min(m.x, float64(monitorWidth)))
	m.y = max(0, min(m.y, float64(monitorHeight)))
	ebiten.SetWindowPosition(int(math.Round(m.x)), int(math.Round(m.y)))

	mx, my := ebiten.CursorPosition()
	x := mx - (height / 2)
	y := my - (width / 2)

	dy, dx := y, x
	if dy < 0 {
		dy = -dy
	}
	if dx < 0 {
		dx = -dx
	}

	m.distance = dx + dy
	if m.distance < width || m.waiting {
		m.stayIdle()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			m.waiting = !m.waiting
		}
		return nil
	}

	if m.state >= 13 {
		playSound(mSound["awake"])
	}
	m.catchCursor(x, y)
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

func (m *neko) catchCursor(x, y int) {
	m.state = 0
	m.min = 8
	m.max = 16

	r := math.Atan2(float64(y), float64(x))
	a := math.Mod((r/math.Pi*180)+360, 360) 

	switch {
	case a <= 292.5 && a > 247.5: 
		m.y -= cfg.Speed
		m.sprite = "up"
	case a <= 337.5 && a > 292.5: 
		m.x += cfg.Speed / math.Sqrt2
		m.y -= cfg.Speed / math.Sqrt2
		m.sprite = "upright"
	case a <= 22.5 || a > 337.5: 
		m.x += cfg.Speed
		m.sprite = "right"
	case a <= 67.5 && a > 22.5: 
		m.x += cfg.Speed / math.Sqrt2
		m.y += cfg.Speed / math.Sqrt2
		m.sprite = "downright"
	case a <= 112.5 && a > 67.5: 
		m.y += cfg.Speed
		m.sprite = "down"
	case a <= 157.5 && a > 112.5: 
		m.x -= cfg.Speed / math.Sqrt2
		m.y += cfg.Speed / math.Sqrt2
		m.sprite = "downleft"
	case a <= 202.5 && a > 157.5: 
		m.x -= cfg.Speed
		m.sprite = "left"
	case a <= 247.5 && a > 202.5: 
		m.x -= cfg.Speed
		m.y -= cfg.Speed
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
	screen.DrawImage(m.img, nil)
}

// System Tray Logic
func onReady() {
	// Icon laden uit assets
	iconData, _ := f.ReadFile("assets/awake.png")
	systray.SetIcon(iconData)
	systray.SetTitle("Neko")
	systray.SetTooltip("Neko - The Cat")

	// Menu items
	mSpeed := systray.AddMenuItem("Snelheid", "Pas snelheid aan")
	mSpeedSlow := mSpeed.AddSubMenuItem("Langzaam", "Slow speed")
	mSpeedNormal := mSpeed.AddSubMenuItem("Normaal", "Normal speed")
	mSpeedFast := mSpeed.AddSubMenuItem("Zoomies!", "Fast speed")

	mScale := systray.AddMenuItem("Grootte", "Pas grootte aan")
	mScaleSmall := mScale.AddSubMenuItem("Klein (1x)", "Small")
	mScaleNormal := mScale.AddSubMenuItem("Normaal (2x)", "Normal")
	mScaleBig := mScale.AddSubMenuItem("Groot (3x)", "Big")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Afsluiten", "Bye bye Neko")

	// Handle clicks in een loop
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
			case <-mSpeedSlow.ClickedCh:
				cfg.Speed = 1.0
			case <-mSpeedNormal.ClickedCh:
				cfg.Speed = 2.0
			case <-mSpeedFast.ClickedCh:
				cfg.Speed = 4.0
			case <-mScaleSmall.ClickedCh:
				updateScale(1.0)
			case <-mScaleNormal.ClickedCh:
				updateScale(2.0)
			case <-mScaleBig.ClickedCh:
				updateScale(3.0)
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

func main() {
	config.PrefixEnv = "NEKO"
	config.File = "neko.ini"
	config.Parse(cfg)

	// Start systray in een aparte goroutine (werkt het best op Windows)
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

	n := &neko{
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
	ebiten.SetWindowMousePassthrough(cfg.MousePassthrough)
	ebiten.SetWindowSize(int(float64(width)*cfg.Scale), int(float64(height)*cfg.Scale))
	ebiten.SetWindowTitle("Neko")

	err := ebiten.RunGameWithOptions(n, &ebiten.RunGameOptions{
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
