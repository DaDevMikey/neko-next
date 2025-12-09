# Neko Next

![Neko](https://raw.githubusercontent.com/crgimenes/neko/master/assets/awake.png)

**Neko Next** is an enhanced fork of the classic desktop cat with a modern system tray interface! [Neko](https://en.wikipedia.org/wiki/Neko_(software)) is a cat that chases the mouse cursor across the screen, an app written in the late 1980s and ported for many platforms.

![Neko](https://github.com/crgimenes/neko/blob/master/fixtures/neko.gif)

This enhanced version adds convenient system tray controls, customization options, and quality-of-life features while maintaining the nostalgic charm of the original.

## ✨ Features

Neko Next includes a convenient **system tray menu** with the following options:

- **Sleep Toggle** - Manually put Neko to sleep or wake them up
- **Teleport to Mouse** - Instantly bring Neko to your cursor position
- **Stay on Primary** - Automatically put Neko to sleep when your mouse leaves the primary monitor (Windows)
- **Click Through Mode** - Let mouse clicks pass through Neko
- **Sound Toggle** - Enable or disable Neko's sounds
- **Adjustable Speed**
  - Slow (1x)
  - Normal (2x) - Default
  - Zoomies! (4x)
  - Ludicrous! (8x)
- **Adjustable Size**
  - Small (1x)
  - Normal (2x) - Default
  - Big (3x)
- **Adjustable Opacity**
  - Solid (100%) - Default
  - Ghost (50%)
  - Ninja (20%)
- **Run on Startup** - Automatically start Neko Next with Windows
- **Settings Persistence** - All your preferences are saved to `neko_settings.json`

## 🚀 Installation

### Download Pre-built Binary
Download the latest release from the [Releases](https://github.com/DaDevMikey/neko-next/releases) page.

### Build from Source

**Prerequisites:**
- Go 1.24+ installed on your system
- CGO enabled (required for Ebiten)

**Windows:**
```bash
set CGO_ENABLED=1
go build -o neko.exe main.go
```

**Linux/macOS:**
```bash
export CGO_ENABLED=1
go build -o neko main.go
```

## 🎮 Usage

Simply run the executable! Neko will appear on your screen and start chasing your mouse cursor. 

- **System Tray Icon** - Right-click the cat icon in your system tray to access all settings
- **Quit** - Select "Exit" from the system tray menu to close Neko

All settings are automatically saved and will be restored the next time you run Neko.

## 🛠️ Development

**Run from source:**
```bash
export CGO_ENABLED=1
go run main.go
```

**Install globally:**
```bash
cd neko-next
go mod tidy
go install
```

Make sure your Go bin directory is in your `$PATH`:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

## 🎨 About

This code is a re-implementation using Golang and **has no relationship to the original software**. This version does not use any part of the original source code except sprites and sounds.

Built with [Ebitengine](https://ebitengine.org), an incredibly easy-to-use gaming library with a vibrant community.

## 🙏 Credits

- Original Neko concept from the late 1980s
- Base implementation inspired by [crgimenes/neko](https://github.com/crgimenes/neko)
- Enhanced with system tray features by DaDevMikey

## 🤝 Contributing

Please follow our [contribution guide](CONTRIBUTING.md).

## 📝 License

See [LICENSE](LICENSE) file for details.
