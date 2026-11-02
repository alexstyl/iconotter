package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

// The flat ICNS composition is separate from Icon Composer's full-bleed layers.
// These are our classic Dock composition defaults, not Liquid Glass requirements.
const macCanvas = 1024
const macTile = 824

type macIconSlot struct {
	points, scale int
	code          string
}

var macIconSlots = []macIconSlot{
	{16, 1, "icp4"}, {16, 2, "ic11"}, {32, 1, "icp5"}, {32, 2, "ic12"},
	{128, 1, "ic07"}, {128, 2, "ic13"}, {256, 1, "ic08"}, {256, 2, "ic14"},
	{512, 1, "ic09"}, {512, 2, "ic10"},
}

func writeMacOS(ctx context.Context, opts options) error {
	if err := validatePadding(opts.Padding); err != nil {
		return err
	}
	if !assetName.MatchString(opts.Name) {
		return errors.New("Choose a macOS icon name starting with a letter, using only letters, digits, underscores, and hyphens.")
	}
	style := opts.MacOSStyle
	if style == "" {
		style = "glass"
	}
	if style != "glass" && style != "flat" {
		return errors.New("Choose glass or flat with --macos-style.")
	}
	var tool string
	if style == "glass" {
		var err error
		tool, err = findIconComposerTool()
		if err != nil {
			return err
		}
	}
	background, err := parseColor(opts.Background)
	if err != nil {
		return err
	}
	artwork, err := loadArtwork(ctx, opts.Foreground)
	if err != nil {
		return err
	}
	var master *image.NRGBA
	var glass *macGlassRenderer
	if style == "glass" {
		document, err := writeMacGlassDocument(ctx, opts, artwork, background)
		if err != nil {
			return err
		}
		scratch, err := os.MkdirTemp("", "iconotter-glass-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(scratch)
		glass = &macGlassRenderer{tool: tool, document: document, scratch: scratch, cache: map[int]*image.NRGBA{}}
		master, err = glass.render(ctx, macCanvas)
		if err != nil {
			return err
		}
	} else {
		master = composeMacIcon(artwork, background, opts.Padding)
	}
	var chunks bytes.Buffer
	catalog := []map[string]string{}
	set := opts.Name + ".iconset"
	assetSet := filepath.Join("Assets.xcassets", opts.Name+".appiconset")
	for _, slot := range macIconSlots {
		if err := ctx.Err(); err != nil {
			return errors.New("Canceled.")
		}
		size := slot.points * slot.scale
		icon := image.NewNRGBA(image.Rect(0, 0, size, size))
		if glass != nil {
			icon, err = glass.render(ctx, size)
			if err != nil {
				return err
			}
		} else {
			xdraw.CatmullRom.Scale(icon, icon.Bounds(), master, master.Bounds(), draw.Src, nil)
		}
		suffix := ""
		if slot.scale == 2 {
			suffix = "@2x"
		}
		filename := fmt.Sprintf("icon_%dx%d%s.png", slot.points, slot.points, suffix)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, icon); err != nil {
			return err
		}
		for _, directory := range []string{set, assetSet} {
			if err := writeBundleFile(ctx, opts.Output, filepath.Join(directory, filename), encoded.Bytes()); err != nil {
				return err
			}
		}
		chunks.WriteString(slot.code)
		if err := binary.Write(&chunks, binary.BigEndian, uint32(encoded.Len()+8)); err != nil {
			return err
		}
		chunks.Write(encoded.Bytes())
		catalog = append(catalog, map[string]string{"filename": filename, "idiom": "mac", "size": fmt.Sprintf("%dx%d", slot.points, slot.points), "scale": fmt.Sprintf("%dx", slot.scale)})
	}
	var icns bytes.Buffer
	icns.WriteString("icns")
	if err := binary.Write(&icns, binary.BigEndian, uint32(chunks.Len()+8)); err != nil {
		return err
	}
	icns.Write(chunks.Bytes())
	if err := writeBundleFile(ctx, opts.Output, opts.Name+".icns", icns.Bytes()); err != nil {
		return err
	}
	contents := map[string]any{"images": catalog, "info": map[string]any{"author": "xcode", "version": 1}}
	data, err := json.MarshalIndent(contents, "", "  ")
	if err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, filepath.Join(assetSet, "Contents.json"), append(data, '\n')); err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, "Assets.xcassets/Contents.json", []byte("{\"info\":{\"author\":\"xcode\",\"version\":1}}\n")); err != nil {
		return err
	}
	if err := writePNGFile(ctx, opts.Output, "preview.png", master); err != nil {
		return err
	}
	guide := fmt.Sprintf(`# macOS app icon

Use %s.icns as your macOS application icon. In Compose Desktop, set nativeDistributions.macOS.iconFile to this file in Gradle. For an Xcode app, add Assets.xcassets/%s.appiconset to your asset catalog and select %s as the app icon.

The iconset contains 16, 32, 128, 256 and 512pt representations at 1x and 2x, up to 1024px. The ICNS is generated locally without requiring Xcode or iconutil.

This is a flattened, classic ICNS export, not an Icon Composer document. Its composition default is an 824px continuous rounded-square approximation centered on a 1024px transparent canvas, with 100px outer margins. This frame is a tool convention for flat Dock icons, not a numeric requirement in Apple's current layered-icon HIG. Artwork defaults to 75%% of the tile's longest side. --padding changes artwork padding inside the tile; it never removes the outer canvas margin. Negative padding intentionally crops artwork at the tile edge. Source transparent padding is trimmed.

No lighting effects or shadows are added. Inspect small representations in the Dock. Current Liquid Glass icons use unmasked, full-bleed layers and system effects through Icon Composer; this export does not generate those layers or appearance variants.

Apple's iconset sizes and naming:
https://developer.apple.com/library/archive/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Optimizing/Optimizing.html
Current icon design guidance and production templates:
https://developer.apple.com/design/human-interface-guidelines/app-icons
https://developer.apple.com/design/resources/
`, opts.Name, opts.Name, opts.Name)
	if style == "glass" {
		guide = fmt.Sprintf(`# macOS glass app icon

Use %s.icns as your macOS app icon, or add Assets.xcassets/%s.appiconset to Xcode. Compose Desktop can select the ICNS using nativeDistributions.macOS.iconFile in Gradle.

Apple's Icon Composer renderer applies glass to the background. The foreground has glass, translucency and shadow disabled. Its own colors and transparency are preserved. The editable %s.icon document is included.

Every unique iconset size is rendered by Apple, then centered on a transparent canvas with the same 824/1024 tile proportion as the flat export. --padding controls foreground spacing inside the tile. The ICNS and PNG assets capture the Default appearance; they do not dynamically change with system appearance. To support layered appearance variants, integrate the .icon document into an Xcode build.

Apple's renderer is required; install Icon Composer on macOS. Choose --macos-style flat for local flat icons without that tool. Glass rendering is never simulated or silently replaced with flat artwork.

https://developer.apple.com/icon-composer/
https://developer.apple.com/library/archive/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Optimizing/Optimizing.html
`, opts.Name, opts.Name, opts.Name)
	}
	return writeBundleFile(ctx, opts.Output, "README.md", []byte(guide))
}

func composeMacIcon(artwork *image.NRGBA, background color.NRGBA, padding *float64) *image.NRGBA {
	icon := image.NewNRGBA(image.Rect(0, 0, macCanvas, macCanvas))
	draw.Draw(icon, icon.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	bounds := paintedBounds(artwork)
	fraction := .75
	if padding != nil {
		fraction = paddingScale(*padding)
	}
	scale := float64(macTile) * fraction / float64(max(bounds.Dx(), bounds.Dy()))
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	target := image.Rect((macCanvas-width)/2, (macCanvas-height)/2, (macCanvas-width)/2+width, (macCanvas-height)/2+height)
	xdraw.CatmullRom.Scale(icon, target, artwork, bounds, draw.Over, nil)
	// Supersample the squircle boundary so small/downsampled icons retain alpha edges.
	half := float64(macTile) / 2
	for y := 0; y < macCanvas; y++ {
		for x := 0; x < macCanvas; x++ {
			inside := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					dx := math.Abs((float64(x) + (float64(sx)+.5)/4 - macCanvas/2) / half)
					dy := math.Abs((float64(y) + (float64(sy)+.5)/4 - macCanvas/2) / half)
					if dx*dx*dx*dx*dx+dy*dy*dy*dy*dy <= 1 {
						inside++
					}
				}
			}
			offset := icon.PixOffset(x, y)
			icon.Pix[offset+3] = uint8((int(icon.Pix[offset+3])*inside + 8) / 16)
			if inside == 0 {
				icon.Pix[offset] = 0
				icon.Pix[offset+1] = 0
				icon.Pix[offset+2] = 0
			}
		}
	}
	return icon
}
