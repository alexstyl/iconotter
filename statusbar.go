package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

func generateStatusBar(ctx context.Context, opts options, output string) (string, error) {
	if opts.MinSDK < 11 {
		return "", errors.New("Status-bar icons require Android API 11 or newer. Use --min-sdk 11 or higher.")
	}
	if math.IsNaN(opts.MarkSize) || math.IsInf(opts.MarkSize, 0) || opts.MarkSize < 1 || opts.MarkSize > 24 {
		return "", errors.New("Choose a status-bar mark size between 1 and 24dp with --size.")
	}
	source := opts.Foreground
	if opts.Monochrome != "" {
		source = opts.Monochrome
	}
	mark, err := loadArtwork(ctx, source)
	if err != nil {
		return "", err
	}
	if !singleColor(mark) {
		return "", errors.New("Status-bar icons need a single-color silhouette. Supply one with --monochrome FILE.")
	}
	if mark.Opaque() {
		return "", errors.New("Choose artwork with transparency around the mark. Opaque artwork becomes a solid rectangle in the status bar.")
	}
	return publishBundle(ctx, output, func(root string) error {
		for _, d := range densities {
			// A status-bar icon is 24dp; launcher legacy icons are 48dp.
			pixels := d.legacy / 2
			if err := writePNGFile(ctx, root, "res/drawable-"+d.name+"/"+opts.Name+".png", rgbaPNG{statusBarLayer(mark, pixels, opts.MarkSize)}); err != nil {
				return err
			}
		}
		preview := image.NewNRGBA(image.Rect(0, 0, 384, 192))
		draw.Draw(preview, image.Rect(0, 0, 192, 192), image.Black, image.Point{}, draw.Src)
		draw.Draw(preview, image.Rect(192, 0, 384, 192), image.White, image.Point{}, draw.Src)
		white := statusBarLayer(mark, 96, opts.MarkSize)
		black := silhouette(white)
		draw.Draw(preview, image.Rect(48, 48, 144, 144), white, image.Point{}, draw.Over)
		draw.Draw(preview, image.Rect(240, 48, 336, 144), black, image.Point{}, draw.Over)
		if err := writePNGFile(ctx, root, "preview.png", preview); err != nil {
			return err
		}
		guide := fmt.Sprintf("# Android status-bar icons\n\nCopy `res/` into your app's resource source set. Set the notification's small icon with `.setSmallIcon(R.drawable.%s)`. Review existing resources with the same name before copying.\n\nThe resources are 32-bit PNGs containing white pixels and transparency on a 24dp canvas. Painted bounds are centered and fit within %gdp on the longest side. Densities: ldpi 18px, mdpi 24px, hdpi 36px, xhdpi 48px, xxhdpi 72px, xxxhdpi 96px. This bundle targets minSdk %d and supports API 11 and newer.\n\nAndroid controls the displayed tint and may resize or dim the icon. `preview.png` shows enlarged white-on-black and example black-on-white renderings. These are notification small icons, not launcher icons. No manifest icon changes are needed.\n\nOfficial guidance:\n- https://developer.android.com/studio/write/create-app-icons\n- https://developer.android.com/develop/ui/compose/notifications/create-notification\n- https://android.googlesource.com/platform/frameworks/base/+/cf89019e500441d8cb4f7cff7f0b07049515c97a/docs/html/guide/practices/ui_guidelines/icon_design_status_bar.jd\n", opts.Name, opts.MarkSize, opts.MinSDK)
		return writeBundleFile(ctx, root, "README.md", []byte(guide))
	})
}

func statusBarLayer(mark *image.NRGBA, size int, markDP float64) *image.NRGBA {
	bounds := paintedBounds(mark)
	scale := markDP / 24 * float64(size) / float64(max(bounds.Dx(), bounds.Dy()))
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	target := image.Rect((size-width)/2, (size-height)/2, (size-width)/2+width, (size-height)/2+height)
	xdraw.CatmullRom.Scale(result, target, mark, bounds, draw.Src, nil)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			alpha := result.NRGBAAt(x, y).A
			result.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
		}
	}
	return result
}
