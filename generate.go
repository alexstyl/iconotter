package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	xdraw "golang.org/x/image/draw"
)

type options struct {
	Foreground, Monochrome, Background, Output, Name string
	Type                                             string
	MacOSStyle                                       string
	BackgroundSet                                    bool
	Platforms                                        []string
	MinSDK                                           int
	MarkSize                                         float64
	Padding                                          *float64
}

type density struct {
	name          string
	layer, legacy int
}

var densities = []density{{"ldpi", 81, 36}, {"mdpi", 108, 48}, {"hdpi", 162, 72}, {"xhdpi", 216, 96}, {"xxhdpi", 324, 144}, {"xxxhdpi", 432, 192}}
var resourceName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func generateAndroid(ctx context.Context, opts options) (string, error) {
	if err := validatePadding(opts.Padding); err != nil {
		return "", err
	}
	if !resourceName.MatchString(opts.Name) {
		return "", errors.New("Choose a resource name starting with a lowercase letter, using only lowercase letters, digits, and underscores.")
	}
	if opts.MinSDK < 1 {
		return "", errors.New("Choose a positive Android API level with --min-sdk.")
	}
	if strings.TrimSpace(opts.Output) == "" {
		return "", errors.New("Choose a new output folder with --output DIR.")
	}
	output, err := filepath.Abs(opts.Output)
	if err != nil {
		return "", err
	}
	if err := newDestination(output); err != nil {
		return "", err
	}
	if opts.Type == "notification" {
		return generateStatusBar(ctx, opts, output)
	}
	if opts.Type != "" && opts.Type != "launcher" {
		return "", errors.New("Choose launcher or notification.")
	}
	if math.IsNaN(opts.MarkSize) || math.IsInf(opts.MarkSize, 0) || opts.MarkSize < 48 || opts.MarkSize > 66 {
		return "", errors.New("Choose a mark size between 48 and 66dp with --size.")
	}
	background, err := loadBackground(ctx, opts.Background)
	if err != nil {
		return "", err
	}
	foreground, err := loadArtwork(ctx, opts.Foreground)
	if err != nil {
		return "", err
	}
	var monochrome *image.NRGBA
	if opts.Monochrome != "" {
		monochrome, err = loadArtwork(ctx, opts.Monochrome)
		if err != nil {
			return "", err
		}
		if !singleColor(monochrome) {
			return "", errors.New("The themed artwork must use a single color and transparency. Choose a silhouette with --monochrome FILE.")
		}
	} else {
		if !singleColor(foreground) {
			return "", errors.New("This artwork uses multiple colors. Supply a transparent, single-color silhouette with --monochrome FILE for themed icons.")
		}
		monochrome = foreground
	}
	monochrome = silhouette(monochrome)
	if err := ctx.Err(); err != nil {
		return "", errors.New("Canceled.")
	}
	return publishBundle(ctx, output, func(root string) error { return writeBundle(ctx, root, opts, foreground, monochrome, background) })
}

func publishBundle(ctx context.Context, output string, write func(string) error) (string, error) {
	// Build beside the destination and publish only after every file succeeds.
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", fmt.Errorf("Could not create the output folder: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(output), ".iconotter-*")
	if err != nil {
		return "", fmt.Errorf("Could not create the output folder: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := write(stage); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", errors.New("Canceled.")
	}
	if err := newDestination(output); err != nil {
		return "", err
	}
	if err := os.Chmod(stage, 0755); err != nil {
		return "", err
	}
	if err := os.Rename(stage, output); err != nil {
		return "", fmt.Errorf("Could not save your icons: %w", err)
	}
	return output, nil
}

func newDestination(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("The output folder %q already exists. Choose a new folder with --output DIR.", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Could not access the output folder: %w", err)
	}
	return nil
}

// Keep the alpha channel in the Play Store PNG even though every pixel is
// opaque: Google Play specifies a 32-bit PNG, and png.Encode otherwise uses RGB.
type rgbaPNG struct{ image.Image }

func (rgbaPNG) Opaque() bool { return false }

func writeBundle(ctx context.Context, root string, opts options, foreground, monochrome *image.NRGBA, background iconBackground) error {
	markDP := opts.MarkSize
	monoDP := opts.MarkSize
	if opts.Padding != nil {
		markDP = paddedMarkSize(foreground, *opts.Padding)
		monoDP = paddedMarkSize(monochrome, *opts.Padding)
	}
	write := func(path string, data []byte) error { return writeBundleFile(ctx, root, path, data) }
	writePNG := func(path string, img image.Image) error { return writePNGFile(ctx, root, path, img) }
	// A color background is a shape drawable; an image becomes a full 108dp layer per density.
	if background.image == nil {
		bgXML := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<shape xmlns:android=\"http://schemas.android.com/apk/res/android\" android:shape=\"rectangle\">\n    <solid android:color=\"#%02X%02X%02X\" />\n</shape>\n", background.color.R, background.color.G, background.color.B)
		if err := write("res/drawable/"+opts.Name+"_background.xml", []byte(bgXML)); err != nil {
			return err
		}
	}
	for _, d := range densities {
		if background.image != nil {
			if err := writePNG("res/drawable-"+d.name+"/"+opts.Name+"_background.png", background.canvas(d.layer)); err != nil {
				return err
			}
		}
		for suffix, img := range map[string]*image.NRGBA{"foreground": foreground, "monochrome": monochrome} {
			size := markDP
			if suffix == "monochrome" {
				size = monoDP
			}
			if err := writePNG("res/drawable-"+d.name+"/"+opts.Name+"_"+suffix+".png", layer(img, d.layer, size)); err != nil {
				return err
			}
		}
		if opts.MinSDK < 26 {
			for _, round := range []bool{false, true} {
				suffix, shape := "", "rounded"
				if round {
					suffix, shape = "_round", "circle"
				}
				if err := writePNG("res/mipmap-"+d.name+"/"+opts.Name+suffix+".png", composite(foreground, background, d.legacy, markDP, shape)); err != nil {
					return err
				}
			}
		}
	}
	for _, version := range []int{26, 33} {
		mono := ""
		if version == 33 {
			mono = fmt.Sprintf("    <monochrome android:drawable=\"@drawable/%s_monochrome\" />\n", opts.Name)
		}
		adaptive := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<adaptive-icon xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <background android:drawable=\"@drawable/%s_background\" />\n    <foreground android:drawable=\"@drawable/%s_foreground\" />\n%s</adaptive-icon>\n", opts.Name, opts.Name, mono)
		for _, suffix := range []string{"", "_round"} {
			if err := write(fmt.Sprintf("res/mipmap-anydpi-v%d/%s%s.xml", version, opts.Name, suffix), []byte(adaptive)); err != nil {
				return err
			}
		}
	}
	manifest := fmt.Sprintf("<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <application\n        android:icon=\"@mipmap/%s\"\n        android:roundIcon=\"@mipmap/%s_round\" />\n</manifest>\n", opts.Name, opts.Name)
	if err := write("manifest-snippet.xml", []byte(manifest)); err != nil {
		return err
	}
	if err := writePNG("play-store.png", rgbaPNG{composite(foreground, background, 512, markDP, "square")}); err != nil {
		return err
	}
	// Show the same composition under common masks and example themed colors.
	preview := image.NewNRGBA(image.Rect(0, 0, 3*256, 2*256))
	tint := color.NRGBA{R: 37, G: 68, B: 52, A: 255}
	themed := image.NewNRGBA(monochrome.Bounds())
	for y := 0; y < themed.Bounds().Dy(); y++ {
		for x := 0; x < themed.Bounds().Dx(); x++ {
			p := tint
			p.A = monochrome.NRGBAAt(x, y).A
			themed.SetNRGBA(x, y, p)
		}
	}
	for i, shape := range []string{"circle", "rounded", "square"} {
		normal := composite(foreground, background, 256, markDP, shape)
		draw.Draw(preview, image.Rect(i*256, 0, (i+1)*256, 256), normal, image.Point{}, draw.Src)

		theme := composite(themed, iconBackground{color: color.NRGBA{R: 212, G: 237, B: 216, A: 255}}, 256, monoDP, shape)
		draw.Draw(preview, image.Rect(i*256, 256, (i+1)*256, 512), theme, image.Point{}, draw.Src)
	}
	if err := writePNG("preview.png", preview); err != nil {
		return err
	}
	composition := fmt.Sprintf("the painted mark centered within a %gdp bounding diagonal inside the 66dp safe area", opts.MarkSize)
	if opts.Padding != nil {
		composition = fmt.Sprintf("%g%% content padding per side of the visible 72dp viewport; negative padding intentionally crops artwork", *opts.Padding)
	}
	backgroundNote := "The opaque background fills the canvas."
	if background.image != nil {
		backgroundNote = "The background image covers the whole 108dp layer. Launchers show about its middle two thirds and reveal more of it as the icon moves."
	}
	guide := fmt.Sprintf("# Android app icons\n\nCopy `res/` into your app's resource source set, preserving the folders. Merge the two application attributes from `manifest-snippet.xml` into your existing AndroidManifest.xml; do not replace your manifest. Review existing icons with the same names, including XML, WebP and PNG variants, before copying. Remove obsolete resources with those names to avoid duplicate or overriding icons.\n\nCompile with Android API 33 or newer. This bundle targets minSdk %d.\n\n- API 26+: 108dp adaptive layers, with %s. %s\n- API 33+: a separate tintable monochrome layer. The launcher must support themed icons and the user must enable them.\n- API 25 and below: density-specific legacy and round PNGs are included when minSdk is below 26.\n\n`preview.png` shows circle, rounded-square and square masks, followed by example themed icons. These are approximations; launcher masks and effects vary. `play-store.png` is an opaque 512×512 PNG without baked-in corner rounding or shadows.\n\nTransparent source padding is trimmed before fitting the mark. Supply clean artwork without baked-in launcher masks or shadows. Thin or unusually shaped marks may need a size adjustment; verify visual balance on your target launcher.\n\nOfficial requirements:\n- https://developer.android.com/develop/ui/compose/system/icon_design_adaptive\n- https://developer.android.com/studio/write/create-app-icons\n- https://developer.android.com/distribute/google-play/resources/icon-design-specifications\n", opts.MinSDK, composition, backgroundNote)
	return write("README.md", []byte(guide))
}

// The launcher displays the inner 72dp of a 108dp layer. Legacy and store
// images bake that composition into their finished canvas; adaptive layers do not.
func composite(mark *image.NRGBA, bg iconBackground, size int, markDP float64, shape string) *image.NRGBA {
	fullSize := size * 2
	foreground := layer(mark, fullSize, markDP)
	full := bg.canvas(fullSize)
	draw.Draw(full, full.Bounds(), foreground, image.Point{}, draw.Over)
	inset := fullSize / 6
	crop := image.Rect(inset, inset, fullSize-inset, fullSize-inset)
	// Supersample mask edges for small legacy sizes.
	result := image.NewNRGBA(image.Rect(0, 0, size*4, size*4))
	xdraw.CatmullRom.Scale(result, result.Bounds(), full, crop, draw.Src, nil)
	if shape != "square" {
		n := float64(result.Bounds().Dx())
		radius := n * .2
		for y := 0; y < int(n); y++ {
			for x := 0; x < int(n); x++ {
				px, py := float64(x)+.5, float64(y)+.5
				inside := true
				if shape == "circle" {
					inside = math.Hypot(px-n/2, py-n/2) <= n/2
				} else {
					dx := math.Max(math.Abs(px-n/2)-(n/2-radius), 0)
					dy := math.Max(math.Abs(py-n/2)-(n/2-radius), 0)
					inside = dx*dx+dy*dy <= radius*radius
				}
				if !inside {
					result.SetNRGBA(x, y, color.NRGBA{})
				}
			}
		}
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), result, result.Bounds(), draw.Src, nil)
	return scaled
}

func writeBundleFile(ctx context.Context, root, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return errors.New("Canceled.")
	}
	target := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0644)
}

func writePNGFile(ctx context.Context, root, path string, img image.Image) error {
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		return err
	}
	return writeBundleFile(ctx, root, path, data.Bytes())
}
