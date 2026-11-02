package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Render SVG backgrounds large enough for the biggest canvas they fill.
const backgroundRenderSize = 2048

// iconBackground is the opaque layer behind the artwork: a solid color, or an
// image that covers the canvas.
type iconBackground struct {
	color color.NRGBA
	image *image.NRGBA
}

// loadBackground reads --background: a #RRGGBB color or an image file.
func loadBackground(ctx context.Context, value string) (iconBackground, error) {
	if strings.HasPrefix(value, "#") {
		c, err := parseColor(value)
		return iconBackground{color: c}, err
	}
	img, err := decodeImage(ctx, value, "background image", "--background FILE", backgroundRenderSize)
	if err != nil {
		return iconBackground{}, err
	}
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			return iconBackground{}, errors.New("The background image has transparent areas. Choose an opaque image, or a color in #RRGGBB format.")
		}
	}
	return iconBackground{image: img}, nil
}

// fill paints rect. An image is scaled to cover rect and cropped to its center,
// so it is never stretched.
func (b iconBackground) fill(dst draw.Image, rect image.Rectangle) {
	if b.image == nil {
		draw.Draw(dst, rect, image.NewUniform(b.color), image.Point{}, draw.Src)
		return
	}
	source := b.image.Bounds()
	crop := source
	if source.Dx()*rect.Dy() > source.Dy()*rect.Dx() {
		width := source.Dy() * rect.Dx() / rect.Dy()
		crop.Min.X += (source.Dx() - width) / 2
		crop.Max.X = crop.Min.X + width
	} else {
		height := source.Dx() * rect.Dy() / rect.Dx()
		crop.Min.Y += (source.Dy() - height) / 2
		crop.Max.Y = crop.Min.Y + height
	}
	xdraw.CatmullRom.Scale(dst, rect, b.image, crop, draw.Src, nil)
}

// canvas returns a size×size image filled with the background.
func (b iconBackground) canvas(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	b.fill(img, img.Bounds())
	return img
}
