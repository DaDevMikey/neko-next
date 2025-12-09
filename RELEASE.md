# Neko Next - Release Notes

## Version 1.2.0 - December 2025

### 🎉 What's New

This release includes significant bug fixes and stability improvements! After the initial 1.0.0 release, we've addressed critical issues reported by users and enhanced the overall experience.

### 🐛 Bug Fixes

#### Fixed System Tray Icon Display
- **Issue**: System tray showed an empty tile instead of the cat icon on Windows
- **Fix**: Converted system tray icon from PNG to proper ICO format
  - Windows system tray requires `.ico` format, not `.png`
  - Added proper `icon.ico` file with error handling
  - System tray now displays the cute cat icon correctly! 🐱

#### Fixed Auto-Wake on Primary Monitor
- **Issue**: When "Stay on Primary" was enabled, the cat wouldn't wake up when mouse returned to the primary monitor
- **Fix**: Reordered logic in the `Update()` function to check monitor state before early returns
  - The wake-up logic was previously unreachable when the cat was sleeping
  - Now correctly detects when mouse returns to primary monitor and wakes the cat
  - Automatically unchecks the "Sleep" menu item when auto-waking

### 📚 Documentation
- Updated README.md with comprehensive feature list
- Improved installation instructions
- Added credits and contribution guidelines

### 🔧 Technical Details

**Files Modified**:
- `main.go` - Fixed system tray icon loading and sleep/wake logic
- `assets/icon.ico` - Added proper Windows icon format
- `README.md` - Complete rewrite for Neko Next features

**Code Improvements**:
- Better error handling for system tray icon loading
- More reliable primary monitor detection and wake-up behavior
- Cleaner separation of manual sleep toggle vs automatic sleep behavior

---

## Version 1.0.0 - December 2025

### 🚀 Initial Release

First public release of **Neko Next** - an enhanced fork of the classic desktop cat!

### ✨ Features

#### System Tray Integration
- System tray icon with convenient dropdown menu
- Easy access to all settings without command-line flags

#### Cat Controls
- **Sleep Toggle** - Manually put Neko to sleep or wake them up
- **Teleport to Mouse** - Instantly bring Neko to your cursor position
- **Stay on Primary** - Automatically put Neko to sleep when your mouse leaves the primary monitor (Windows)

#### Customization Options
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

#### Quality of Life
- **Run on Startup** - Automatically start Neko Next with Windows
- **Settings Persistence** - All preferences saved to `neko_settings.json`
- Settings are automatically restored on next launch

### 🔧 Technical Features
- Built with Go 1.24+
- Uses Ebitengine for rendering
- Windows-specific optimizations
- Global mouse position tracking
- Multi-monitor support
- Transparent window with configurable opacity

### 📝 Known Issues
- Windows-only at this time
- Requires CGO enabled for compilation

---

## Upgrading

To upgrade from 1.0.0 to 1.2.0:
1. Download the latest `neko.exe` from the [Releases](https://github.com/DaDevMikey/neko-next/releases) page
2. Replace your existing `neko.exe` file
3. Your settings in `neko_settings.json` will be preserved

---

## Credits

- Original Neko concept from the late 1980s
- Base implementation inspired by [crgimenes/neko](https://github.com/crgimenes/neko)
- Enhanced with system tray features by [@DaDevMikey](https://github.com/DaDevMikey)
- Built with [Ebitengine](https://ebitengine.org)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute to this project.

## License

See [LICENSE](LICENSE) file for details.
