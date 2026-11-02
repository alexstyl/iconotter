package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"math"
	"strconv"

	xdraw "golang.org/x/image/draw"
)

func writeWeb(ctx context.Context, opts options) error {
	if err := validatePadding(opts.Padding); err != nil {
		return err
	}
	background, err := loadBackground(ctx, opts.Background)
	if err != nil {
		return err
	}
	artwork, err := loadArtwork(ctx, opts.Foreground)
	if err != nil {
		return err
	}
	icons := make([]*image.NRGBA, 0, 3)
	for _, size := range []int{16, 32, 48} {
		icon := composeWebIcon(artwork, background, size, opts.Padding)
		icons = append(icons, icon)
		if size != 48 {
			if err := writePNGFile(ctx, opts.Output, "favicon-"+strconv.Itoa(size)+"x"+strconv.Itoa(size)+".png", icon); err != nil {
				return err
			}
		}
	}
	ico, err := encodeICO(icons)
	if err != nil {
		return err
	}
	if err := writeBundleFile(ctx, opts.Output, "favicon.ico", ico); err != nil {
		return err
	}
	if err := writePNGFile(ctx, opts.Output, "apple-touch-icon.png", composeWebIcon(artwork, background, 180, opts.Padding)); err != nil {
		return err
	}
	return writePNGFile(ctx, opts.Output, "preview.png", composeWebIcon(artwork, background, 256, opts.Padding))
}

func composeWebIcon(artwork *image.NRGBA, background iconBackground, size int, padding *float64) *image.NRGBA {
	icon := background.canvas(size)
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

// ICO stores bottom-up BGRA pixels followed by a one-bit transparency mask.
// The icons are opaque, so the mask is zero. BMP entries work in older browsers.
func encodeICO(icons []*image.NRGBA) ([]byte, error) {
	var output bytes.Buffer
	if err := binary.Write(&output, binary.LittleEndian, []uint16{0, 1, uint16(len(icons))}); err != nil {
		return nil, err
	}
	data := make([][]byte, len(icons))
	offset := 6 + 16*len(icons)
	for i, icon := range icons {
		size := icon.Bounds().Dx()
		var entry bytes.Buffer
		if size == 256 {
			if err := png.Encode(&entry, icon); err != nil {
				return nil, err
			}
		} else {
			maskStride := ((size + 31) / 32) * 4
			header := struct {
				Size, Width, Height                                                                   uint32
				Planes, BitCount                                                                      uint16
				Compression, ImageSize, XPixelsPerMeter, YPixelsPerMeter, ColorsUsed, ImportantColors uint32
			}{40, uint32(size), uint32(size * 2), 1, 32, 0, uint32(size*size*4 + maskStride*size), 0, 0, 0, 0}
			if err := binary.Write(&entry, binary.LittleEndian, header); err != nil {
				return nil, err
			}
			mask := make([]byte, maskStride*size)
			for y := size - 1; y >= 0; y-- {
				for x := 0; x < size; x++ {
					p := icon.NRGBAAt(x, y)
					entry.Write([]byte{p.B, p.G, p.R, p.A})
					if p.A < 128 {
						mask[(size-1-y)*maskStride+x/8] |= 1 << (7 - x%8)
					}
				}
			}
			entry.Write(mask)
		}
		data[i] = entry.Bytes()
		output.Write([]byte{byte(size), byte(size), 0, 0})
		if err := binary.Write(&output, binary.LittleEndian, []uint16{1, 32}); err != nil {
			return nil, err
		}
		if err := binary.Write(&output, binary.LittleEndian, []uint32{uint32(len(data[i])), uint32(offset)}); err != nil {
			return nil, err
		}
		offset += len(data[i])
	}
	for _, entry := range data {
		output.Write(entry)
	}
	return output.Bytes(), nil
}
