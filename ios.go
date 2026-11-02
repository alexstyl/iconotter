package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"path/filepath"
	"regexp"

	xdraw "golang.org/x/image/draw"
)

var assetName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func writeIOS(ctx context.Context, opts options) error {
	if err := validatePadding(opts.Padding); err != nil {
		return err
	}
	if !assetName.MatchString(opts.Name) {
		return errors.New("Choose an iOS asset name starting with a letter, using only letters, digits, underscores, and hyphens.")
	}
	background, err := parseColor(opts.Background)
	if err != nil {
		return err
	}
	artwork, err := loadArtwork(ctx, opts.Foreground)
	if err != nil {
		return err
	}
	const size = 1024
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(icon, icon.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	bounds := paintedBounds(artwork)
	// This is a composition default, not Android's adaptive-icon safe zone.
	// Leave the outside quarter for whitespace while preserving the mark's ratio.
	scale := float64(size) * .75 / float64(max(bounds.Dx(), bounds.Dy()))
	if opts.Padding != nil {
		scale = float64(size) * paddingScale(*opts.Padding) / float64(max(bounds.Dx(), bounds.Dy()))
	}
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	target := image.Rect((size-width)/2, (size-height)/2, (size-width)/2+width, (size-height)/2+height)
	xdraw.CatmullRom.Scale(icon, target, artwork, bounds, draw.Over, nil)
	set := filepath.Join("Assets.xcassets", opts.Name+".appiconset")
	// png.Encode omits the alpha channel for the fully opaque iOS icon.
	if err := writePNGFile(ctx, opts.Output, filepath.Join(set, "icon-1024.png"), icon); err != nil {
		return err
	}
	contents := map[string]any{
		"images": []map[string]string{{"filename": "icon-1024.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"}},
		"info":   map[string]any{"author": "xcode", "version": 1},
	}
	data, err := json.MarshalIndent(contents, "", "  ")
	if err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, filepath.Join(set, "Contents.json"), append(data, '\n')); err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, "Assets.xcassets/Contents.json", []byte("{\"info\":{\"author\":\"xcode\",\"version\":1}}\n")); err != nil {
		return err
	}
	if err := writePNGFile(ctx, opts.Output, "preview.png", icon); err != nil {
		return err
	}
	composition := "fills up to 75% of the canvas on its longest side"
	if opts.Padding != nil {
		composition = fmt.Sprintf("uses %g%% content padding per side; negative padding intentionally crops artwork", *opts.Padding)
	}
	guide := fmt.Sprintf("# iOS app icon\n\nAdd `Assets.xcassets/%s.appiconset` to your app's asset catalog. Select `%s` as the App Icons Source in your target's General settings, or set ASSETCATALOG_COMPILER_APPICON_NAME to `%s`. Review any existing icon set with that name first.\n\nThis is a standard, flattened 1024×1024 PNG app icon, with an opaque background and no alpha channel, pre-rounded corners, or shadows. Xcode's single-size app-icon support generates the smaller iPhone and iPad variants. The mark is centered, preserves its aspect ratio, and %s. `preview.png` shows the unmasked artwork; iOS applies the final mask.\n\nThis export uses an asset catalog. It does not create a layered Icon Composer `.icon` file or custom dark, tinted, or clear variants. Preview system-generated appearances on your target iOS versions.\n\nApple's requirements:\nhttps://developer.apple.com/documentation/xcode/configuring-your-app-icon\nhttps://developer.apple.com/design/human-interface-guidelines/app-icons\n", opts.Name, opts.Name, opts.Name, composition)
	return writeBundleFile(ctx, opts.Output, "README.md", []byte(guide))
}
