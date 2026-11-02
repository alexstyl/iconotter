package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"regexp"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Keep backgrounds at the biggest canvas they fill, the 1024px iOS and macOS
// icons: SVGs render at this size, and larger images are scaled down once
// instead of for every icon.
const backgroundRenderSize = 1024

var hexColor = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

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
	if hexColor.MatchString(value) {
		return iconBackground{}, fmt.Errorf("Write colors with a leading #, such as --background '#%s'.", value)
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
	if shorter := min(img.Bounds().Dx(), img.Bounds().Dy()); shorter > backgroundRenderSize {
		scaled := image.NewNRGBA(image.Rect(0, 0, img.Bounds().Dx()*backgroundRenderSize/shorter, img.Bounds().Dy()*backgroundRenderSize/shorter))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = scaled
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

// adaptiveLayer returns a size×size Android adaptive background layer. Launchers
// show the inner 72dp of the 108dp layer, so an image covers that visible area,
// matching other platforms, and the margin revealed by launcher motion mirrors
// the image's edges.
func (b iconBackground) adaptiveLayer(size int) *image.NRGBA {
	if b.image == nil {
		return b.canvas(size)
	}
	inset := size / 6
	visible := b.canvas(size - 2*inset)
	n := visible.Bounds().Dx()
	mirror := func(v int) int {
		v = ((v % (2 * n)) + 2*n) % (2 * n)
		if v >= n {
			return 2*n - 1 - v
		}
		return v
	}
	layer := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			layer.SetNRGBA(x, y, visible.NRGBAAt(mirror(x-inset), mirror(y-inset)))
		}
	}
	return layer
}
