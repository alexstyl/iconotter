package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"strconv"

	xdraw "golang.org/x/image/draw"
)

func desktopArtwork(ctx context.Context, opts options) (*image.NRGBA, *color.NRGBA, error) {
	if err := validatePadding(opts.Padding); err != nil {
		return nil, nil, err
	}
	if !assetName.MatchString(opts.Name) {
		return nil, nil, errors.New("Choose an icon name starting with a letter, using only letters, digits, underscores, and hyphens.")
	}
	artwork, err := loadArtwork(ctx, opts.Foreground)
	if err != nil {
		return nil, nil, err
	}
	if !opts.BackgroundSet {
		return artwork, nil, nil
	}
	background, err := parseColor(opts.Background)
	return artwork, &background, err
}

func composeDesktopIcon(artwork *image.NRGBA, background *color.NRGBA, size int, padding *float64) *image.NRGBA {
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	if background != nil {
		draw.Draw(icon, icon.Bounds(), image.NewUniform(*background), image.Point{}, draw.Src)
	}
	bounds := paintedBounds(artwork)
	fit := .9
	if padding != nil {
		fit = paddingScale(*padding)
	}
	scale := float64(size) * fit / float64(max(bounds.Dx(), bounds.Dy()))
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	target := image.Rect((size-width)/2, (size-height)/2, (size-width)/2+width, (size-height)/2+height)
	xdraw.CatmullRom.Scale(icon, target, artwork, bounds, draw.Over, nil)
	return icon
}

func writeWindows(ctx context.Context, opts options) error {
	artwork, background, err := desktopArtwork(ctx, opts)
	if err != nil {
		return err
	}
	var icons []*image.NRGBA
	for _, size := range []int{16, 24, 32, 48, 64, 96, 256} {
		icons = append(icons, composeDesktopIcon(artwork, background, size, opts.Padding))
	}
	ico, err := encodeICO(icons)
	if err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, opts.Name+".ico", ico); err != nil {
		return err
	}
	return writePNGFile(ctx, opts.Output, "preview.png", composeDesktopIcon(artwork, background, 256, opts.Padding))
}

func writeLinux(ctx context.Context, opts options) error {
	artwork, background, err := desktopArtwork(ctx, opts)
	if err != nil {
		return err
	}
	for _, size := range []int{16, 24, 32, 48, 64, 128, 256, 512} {
		icon := composeDesktopIcon(artwork, background, size, opts.Padding)
		path := filepath.Join("hicolor", strconv.Itoa(size)+"x"+strconv.Itoa(size), "apps", opts.Name+".png")
		if err := writePNGFile(ctx, opts.Output, path, icon); err != nil {
			return err
		}
		if size == 512 {
			if err := writePNGFile(ctx, opts.Output, opts.Name+".png", icon); err != nil {
				return err
			}
		}
		if size == 256 {
			if err := writePNGFile(ctx, opts.Output, "preview.png", icon); err != nil {
				return err
			}
		}
	}
	return nil
}
