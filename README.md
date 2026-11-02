# Icon Otter

Create every icon your app needs from one piece of artwork, straight from the terminal.

![An otter holding a terminal icon on a yellow background](assets/readme-header.png)

Icon Otter turns an SVG, PNG, or JPEG into ready-to-use app icons for Android, iOS, macOS, Windows, Linux and the web, from adaptive launcher icons to `.icns`, `.ico` and favicons. Everything runs locally, without an account, server, or AI service.

[Installation](#installation) · [Quick start](#quick-start) · [Examples](#examples) · [Options](#options) · [Integration](#integration)

## Installation

With [Homebrew](https://brew.sh) on macOS or Linux:

```sh
brew install alexstyl/tap/iconotter
```

On Windows, or without Homebrew, download the archive for your platform from [GitHub Releases](https://github.com/alexstyl/iconotter/releases/latest), extract it, and move `iconotter` onto your `PATH`.

With Go 1.25.8 or newer:

```sh
go install github.com/alexstyl/iconotter@latest
```

Check the installation with `iconotter --version`.

## Quick start

Supply your own single-color SVG or transparent PNG:

```sh
iconotter launcher --foreground logo.svg --background '#FFFFFF'
```

This creates `app-icons/` with Android resources, an iOS asset catalog, previews, and integration instructions. `launcher` exports Android and iOS unless you [choose other platforms](#choose-a-platform) with `--platform`. For multicolor Android artwork, also supply a [monochrome silhouette](#multicolor-artwork).

```text
app-icons/
├── android/
│   ├── res/
│   ├── manifest-snippet.xml
│   ├── play-store.png
│   ├── preview.png
│   └── README.md
├── ios/
│   ├── Assets.xcassets/AppIcon.appiconset/
│   ├── preview.png
│   └── README.md
└── README.md
```

Generation never overwrites an existing output directory or edits your app. All selected platforms must succeed before the export is published. Copy the generated resources into your native project following the [integration steps](#integration).

## Features

- **Android launcher icons:** adaptive foreground and background layers, themed monochrome icons, legacy and round icons, and a Play Store image.
- **macOS app icons:** Apple-rendered glass backgrounds with flat foregrounds, a multi-resolution `.icns`, complete `.iconset`, Xcode asset catalog, editable `.icon` document, and preview. An explicit flat style works without Apple tools.
- **iOS app icons:** an opaque 1024×1024 PNG in an Xcode asset catalog.
- **Windows and Linux app icons:** a multi-size `.ico`, and PNGs in the freedesktop `hicolor` theme layout.
- **Web favicons:** `favicon.ico`, 16px and 32px PNG favicons, and an Apple touch icon.
- **Android notification icons:** white artwork on a transparent status-bar canvas.
- **Flexible composition:** trim transparent source padding, preserve aspect ratio, choose a solid background, and adjust content padding—including intentional cropping.
- **Local SVG rendering:** embedded resvg handles paths, shapes, gradients, and internal references without an external renderer.
- **Interactive or scripted:** prompts when artwork is omitted in a terminal; explicit flags for scripts and agents.

## Examples

### Choose a platform

```sh
iconotter launcher --foreground logo.svg --platform android --output android-icons
iconotter launcher --foreground illustration.png --platform ios --output ios-icons
iconotter launcher --foreground logo.svg --platform macos --name Studio --output mac-icons
iconotter launcher --foreground logo.svg --platform windows,linux --name my-app --output desktop-icons
iconotter launcher --foreground logo.svg --platform web --output web-icons
```

`--platform` accepts comma-separated values or repeated flags. Duplicate selections are ignored. iOS-only artwork can use multiple colors without a silhouette.

### Multicolor artwork

Android themed icons need a single-color mark. For multicolor artwork, supply a transparent silhouette separately:

```sh
iconotter launcher \
  --foreground character.png \
  --monochrome character-silhouette.svg \
  --background '#85C9B4' \
  --output character-icons
```

Single-color foreground artwork can supply its own monochrome silhouette.

### Remove padding or crop the artwork

```sh
# Fit the longest painted side to the visible icon bounds
iconotter launcher --foreground logo.svg --padding 0 --output unpadded-icons

# Enlarge a character beyond the edges for intentional clipping
iconotter launcher \
  --foreground character.png \
  --monochrome character-silhouette.svg \
  --background '#85C9B4' \
  --padding=-15 \
  --output cropped-icons
```

Padding is a percentage per side of the visible icon canvas. Positive values inset the artwork; zero removes the added inset; negative values enlarge and crop it. Aspect ratio is preserved, so rectangular artwork can still leave space along its shorter side.

On Android, padding is measured against the visible inner 72dp of the 108dp adaptive layer. On iOS, it uses the full 1024px canvas. On macOS, it applies inside the visible 824px tile, independently of the fixed transparent outer margins. Android legacy, store, and preview images use the same visible composition. `--padding` and `--size` cannot be combined.

Without `--padding`, Android uses a 56dp bounding diagonal inside the 66dp safe area; iOS fits the longest painted side within 75% of its canvas; macOS uses 75% of its tile. These foreground sizes are composition defaults, not Apple requirements.

### Android notification icons

```sh
iconotter notification --foreground bell.svg
```

This creates `notification-icons/android/` with density-specific white icons on a transparent 24dp canvas. Supply a transparent single-color mark or a dedicated silhouette with `--monochrome FILE`. Multicolor and fully opaque artwork are rejected unless a suitable silhouette is supplied.

There is no separate iOS notification export: iOS uses the app icon for notifications.

## Options

| Flag | Applies to | Behavior / default |
| --- | --- | --- |
| `--foreground FILE` | Both commands | SVG, PNG, or JPEG artwork; prompts when omitted in an interactive terminal. |
| `--platform LIST` | Both commands | `launcher`: `android,ios` by default; also accepts `macos`, `windows`, `linux` and `web`. `notification`: `android` only. |
| `--output DIR`, `-o DIR` | Both commands | A new directory; defaults to `app-icons` or `notification-icons`. |
| `--name NAME` | Both commands | Android launcher: `ic_launcher`; iOS/macOS/Windows: `AppIcon`; Linux: `app-icon`; notification: `ic_stat_notification`. |
| `--monochrome FILE` | Android | Transparent single-color silhouette for themed or notification icons. |
| `--background '#RRGGBB'` | Launcher | Opaque solid color; defaults to white. Windows and Linux icons keep source transparency unless it is supplied. |
| `--macos-style STYLE` | macOS launcher | `glass` (default): Apple-rendered background with a flat foreground. Requires Icon Composer on macOS. `flat`: local rendering without Apple tools. |
| `--padding NUMBER` | Launcher | Percent per side, from -100 up to (but not including) 50. Omitted: platform-specific sizing defaults. |
| `--size NUMBER` | Android | Launcher bounding diagonal: 48–66dp, default 56. Notification longest side: 1–24dp, default 22. |
| `--min-sdk NUMBER` | Android | Minimum Android API level; defaults to 21. Launcher exports include legacy icons below 26. |

`--monochrome`, `--min-sdk`, and `--size` are rejected when explicitly passed for an export without Android. A custom `--name` for both platforms must start with a lowercase letter and use lowercase letters, digits, or underscores.

For full command help:

```sh
iconotter launcher --help
iconotter notification --help
```

## Integration

Generated resources work with native apps, Flutter, React Native, and Compose Multiplatform through their native platform projects. Each export includes platform-specific instructions.

### Android launcher

Copy `android/res/` into your Android resource source set. Merge these attributes into the existing `<application>` element:

```xml
android:icon="@mipmap/ic_launcher"
android:roundIcon="@mipmap/ic_launcher_round"
```

Compile with Android API 33 or newer. Review old PNG, WebP, XML, and qualified resources with the same names before copying to avoid duplicate resources or stale overrides.

| Android version | Generated resource |
| --- | --- |
| API 25 and below | Legacy launcher and round PNGs, when `--min-sdk` is below 26. |
| API 26–32 | Color adaptive XML in `mipmap-anydpi-v26`, with transparent foreground and opaque background layers. |
| API 33+ | Adaptive XML in `mipmap-anydpi-v33`, also including a tintable monochrome layer. |

Adaptive layers are 108dp; the outer 18dp allows masking and motion. Default sizing keeps artwork within the 66dp safe area. Explicit padding can extend beyond it, so inspect important details under launcher masks. Themed appearance requires a supporting launcher and enabled themed icons.

Exports include ldpi through xxxhdpi: adaptive layers are 81, 108, 162, 216, 324, and 432px; legacy icons are 36, 48, 72, 96, 144, and 192px. The Play Store image is an opaque 512×512, 32-bit PNG without baked-in corner rounding or shadows.

### Android notification

Copy `android/res/` into your Android resource source set, then use:

```kotlin
.setSmallIcon(R.drawable.ic_stat_notification)
```

No launcher manifest changes are needed. Android controls tint and may resize or dim the icon. API 11 and newer are supported; density-specific sizes are 18, 24, 36, 48, 72, and 96px.

### macOS

```sh
iconotter launcher --foreground logo.svg --platform macos --name Studio --output mac-icons
```

Use `mac-icons/macos/Studio.icns` in your application bundle. For Compose Desktop:

```kotlin
compose.desktop {
    application {
        nativeDistributions {
            macOS { iconFile.set(project.file("mac-icons/macos/Studio.icns")) }
        }
    }
}
```

macOS exports default to **glass on the background only**. Apple's installed Icon Composer renderer supplies the material. The foreground's glass, translucency and shadow are disabled; the source artwork keeps its own colors and alpha. Apple's ColorSync converts the renderer's Display P3 output to sRGB before PNG and ICNS encoding.

Install [Icon Composer](https://developer.apple.com/icon-composer/) on macOS, either standalone or with Xcode. Icon Otter finds its bundled `ictool` in `/Applications`, the selected Xcode, or on PATH. Glass rendering never falls back silently or simulates Apple's effects. The bundled CLI is tested with Icon Composer 1.3; future Apple versions may change its document or renderer behavior.

To generate a local flat icon without Apple tools:

```sh
iconotter launcher --foreground logo.svg --platform macos --macos-style flat --output flat-icons
```

Xcode projects can use `macos/Assets.xcassets/Studio.appiconset`. Glass exports also include `Studio.icon`, the editable Icon Composer source. Integrate that document into Xcode to support layered system appearances. The generated `.icns` and PNG assets capture the Default appearance and do not dynamically respond to system appearance changes.

Both styles include `Studio.iconset`, with Apple's documented 16, 32, 128, 256 and 512pt representations at 1x and 2x, plus a 1024px preview. Glass exports render every unique size with Apple’s renderer. ICNS encoding itself is local and does not require `iconutil`.

The exported tile is centered within a 1024px transparent canvas using an 824/1024 tile proportion. The flat style uses a continuous rounded-square approximation; glass uses Apple's rendered shape. The outer margin is a classic Dock composition convention, not a numeric requirement in Apple's current HIG. Foreground artwork is centered, preserves its aspect ratio and defaults to 75% of the tile's longest side. `--padding` changes foreground spacing inside the tile while retaining the outer canvas margin. Supply source artwork without a pre-existing app-icon background to avoid double framing.

Apple's current HIG specifies a 1024px canvas and full-bleed, unmasked source layers for Icon Composer. Icon Otter's `.icon` document follows that source layout; the transparent outer margin is added only to the flat PNG/ICNS exports. Custom dark, clear and tinted exports are not generated.

### iOS

Add `ios/Assets.xcassets/AppIcon.appiconset` to your app's asset catalog, then select **AppIcon** as the target's App Icons Source.

The icon is a flattened, opaque 1024×1024 PNG without an alpha channel, pre-rounded corners, or shadows. Xcode's single-size asset catalog support generates smaller iPhone and iPad variants; iOS applies the final mask. The preview is unmasked.

### Windows

Use `windows/AppIcon.ico` as your application icon, for example as the executable's icon resource or your installer's icon. It contains 16, 24, 32, 48, 64, 96 and 256px images.

### Linux

Use `linux/app-icon.png` (512px) as your application icon. For desktop menus, install the files from `linux/hicolor/` into the `hicolor` icon theme, such as `~/.local/share/icons/hicolor/`, and set `Icon=app-icon` in your `.desktop` file.

### Web

Copy `favicon.ico`, `favicon-16x16.png`, `favicon-32x32.png` and `apple-touch-icon.png` from `web/` into your site's public root, then link them from your page's `<head>`:

```html
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
```

## Limitations and troubleshooting

- **Backgrounds are solid colors.** `--background` accepts opaque `#RRGGBB` values. Gradients inside SVG foreground artwork are supported; gradient backgrounds are not.
- **SVG inputs are rasterized.** All platforms receive raster artwork, not Android vector drawables. SVGs require a `viewBox`; convert text to paths. Linked files, embedded images, scripts, and stylesheets are rejected.
- **Existing image backgrounds remain.** Transparent padding is trimmed, but white or colored backgrounds in PNG/JPEG inputs are not automatically removed. Use transparent artwork for an isolated foreground.
- **Output directory already exists?** Choose a new `--output` directory. Existing exports are never overwritten.
- **Android icon looks unchanged?** Launchers can cache icons after an update. Check the installed resources before assuming export failed; uninstalling to refresh the cache can remove app data.
- **Apple appearance variants:** macOS glass exports include an editable `.icon` document. Exported ICNS and PNG files capture the Default appearance only. iOS exports remain flat asset catalogs. Custom dark, tinted and clear exports are not generated.
- **Input limits:** files must be at most 10 MiB; decoded bitmap images must contain at most 16 million pixels. Preserve attribution required by the artwork's license.

Running a command without artwork in an interactive terminal opens a prompt. Launcher generation also asks for the background color when omitted. For scripts, pass flags explicitly: piped invocations fail immediately when artwork is missing. `NO_COLOR` and `ACCESSIBLE=1` support plain accessible prompts. Results go to stdout; errors go to stderr with a nonzero exit status.

## Development and contributing

From a local checkout:

```sh
go test ./...
go vet ./...
go build -o iconotter .
```

SVG rendering uses [resvg](https://github.com/linebender/resvg) compiled to WebAssembly from `resvg/` and embedded in the binary. After changing `resvg/`, install Rust with the `wasm32-unknown-unknown` target, run `go generate`, and commit the rebuilt `resvg/resvg.wasm` and `resvg/THIRD_PARTY_LICENSES.txt`.

Optional native resource compilation checks:

```sh
ICONOTTER_TEST_ANDROID=1 ANDROID_HOME=/path/to/android/sdk ICONOTTER_TEST_XCODE=1 go test ./...
```

The Android check needs SDK platform 33 and build-tools; the Xcode check needs Xcode with the iOS SDK. Flat icon generation requires neither SDK. macOS glass rendering requires Apple’s Icon Composer; the native rendering test runs when its tool is installed. Tests cover image output, alpha channels, platform selection, padding and cropping, native resource compilation, invalid input, and preserving existing files.

Report reproducible bugs and propose changes through the repository's issues and pull requests. Include the command, expected result, and relevant platform details.

## Platform references

- [Android adaptive icons](https://developer.android.com/develop/ui/compose/system/icon_design_adaptive)
- [Android Studio icon generation](https://developer.android.com/studio/write/create-app-icons)
- [AdaptiveIconDrawable viewport](https://developer.android.com/reference/android/graphics/drawable/AdaptiveIconDrawable)
- [Google Play icon specifications](https://developer.android.com/distribute/google-play/resources/icon-design-specifications)
- [Android notification setup](https://developer.android.com/develop/ui/compose/notifications/create-notification)
- [AOSP notification icon sizes and format](https://android.googlesource.com/platform/frameworks/base/+/cf89019e500441d8cb4f7cff7f0b07049515c97a/docs/html/guide/practices/ui_guidelines/icon_design_status_bar.jd)
- [Apple macOS iconset sizes and naming](https://developer.apple.com/library/archive/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Optimizing/Optimizing.html)
- [Apple app-icon asset catalogs](https://developer.apple.com/documentation/xcode/configuring-your-app-icon)
- [Apple app-icon design](https://developer.apple.com/design/human-interface-guidelines/app-icons)

## License

Icon Otter is released under the [MIT License](LICENSE).
