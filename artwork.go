package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const maxInputBytes = 10 << 20
const maxImagePixels = 16 << 20
const renderSize = 864

func parseColor(value string) (color.NRGBA, error) {
	if len(value) != 7 || value[0] != '#' {
		return color.NRGBA{}, errors.New("Choose an opaque background color in #RRGGBB format.")
	}
	rgb, err := hex.DecodeString(value[1:])
	if err != nil {
		return color.NRGBA{}, errors.New("Choose an opaque background color in #RRGGBB format.")
	}
	return color.NRGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}, nil
}

func loadArtwork(ctx context.Context, path string) (*image.NRGBA, error) {
	img, err := decodeImage(ctx, path, "artwork", "--foreground FILE", renderSize)
	if err != nil {
		return nil, err
	}
	if paintedBounds(img).Empty() {
		return nil, errors.New("The artwork is fully transparent. Choose artwork with a visible mark.")
	}
	return img, nil
}

// decodeImage reads an SVG, PNG or JPEG file, rendering SVG so its longest
// side is svgSize pixels. noun and flag name the file in errors, such as
// "artwork" and "--foreground FILE".
func decodeImage(ctx context.Context, path, noun, flag string, svgSize float64) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Could not open %s %q: %w", noun, path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("Could not read %s %q: %w", noun, path, err)
	}
	if len(data) > maxInputBytes {
		return nil, fmt.Errorf("The %s is too large. Choose a file smaller than 10 MB.", noun)
	}
	var source image.Image
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		normalized, width, height, err := normalizeSVG(data, svgSize)
		if err != nil {
			return nil, fmt.Errorf("Could not use %s %q: %w", noun, path, err)
		}
		source, err = renderSVG(ctx, normalized, width, height)
		if err != nil {
			return nil, fmt.Errorf("Could not render the SVG. Check the %s or export it as PNG.", noun)
		}
	case ".png", ".jpg", ".jpeg":
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, errors.New("Could not read the image. Choose a valid PNG or JPEG file.")
		}
		if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > maxImagePixels {
			return nil, fmt.Errorf("The %s is too large. Choose an image with at most 16 million pixels.", noun)
		}
		source, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, errors.New("Could not read the image. Choose a valid PNG or JPEG file.")
		}
	default:
		return nil, fmt.Errorf("Choose SVG, PNG, or JPEG %s with %s.", noun, flag)
	}
	img := image.NewNRGBA(image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy()))
	draw.Draw(img, img.Bounds(), source, source.Bounds().Min, draw.Src)
	return img, nil
}

// Render SVG in a bounded viewport, with no fonts, scripts or linked files.
func normalizeSVG(data []byte, longest float64) ([]byte, int, int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	depth, roots := 0, 0
	width, height := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, errors.New("Choose a valid SVG file.")
		}
		switch node := token.(type) {
		case xml.Directive:
			return nil, 0, 0, errors.New("Export the SVG without a document type declaration.")
		case xml.ProcInst:
			if node.Target != "xml" {
				return nil, 0, 0, errors.New("Export the SVG without linked stylesheets.")
			}
			continue
		case xml.StartElement:
			if node.Name.Space == "" {
				node.Name.Space = "http://www.w3.org/2000/svg"
			}
			attrs := make([]xml.Attr, 0, len(node.Attr))
			for _, attr := range node.Attr {
				if attr.Name.Local != "xmlns" && attr.Name.Space != "xmlns" {
					attrs = append(attrs, attr)
				}
			}
			node.Attr = attrs
			switch strings.ToLower(node.Name.Local) {
			case "text", "tspan", "textpath":
				return nil, 0, 0, errors.New("Convert text to paths before using this SVG.")
			case "image", "feimage", "script", "foreignobject", "style":
				return nil, 0, 0, errors.New("Use an SVG with inline vector shapes and styles, without images, scripts, or stylesheets.")
			}
			for _, attr := range node.Attr {
				value := strings.ToLower(strings.TrimSpace(attr.Value))
				if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") || (strings.EqualFold(attr.Name.Local, "href") && !strings.HasPrefix(value, "#")) || strings.Contains(value, "url(") && !internalURLs(value) {
					return nil, 0, 0, errors.New("Use self-contained SVG artwork without scripts or linked files.")
				}
			}
			if depth == 0 {
				roots++
				if roots != 1 || node.Name.Local != "svg" {
					return nil, 0, 0, errors.New("Choose a valid SVG file.")
				}
				var viewbox string
				for _, attr := range node.Attr {
					if attr.Name.Local == "viewBox" {
						viewbox = attr.Value
					}
				}
				fields := strings.Fields(strings.ReplaceAll(viewbox, ",", " "))
				if len(fields) != 4 {
					return nil, 0, 0, errors.New("Add a viewBox to the SVG, or export your artwork as PNG.")
				}
				coords := make([]float64, 4)
				for i, field := range fields {
					v, err := strconv.ParseFloat(field, 64)
					if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
						return nil, 0, 0, errors.New("The SVG viewBox is invalid.")
					}
					coords[i] = v
				}
				if coords[2] <= 0 || coords[3] <= 0 {
					return nil, 0, 0, errors.New("The SVG viewBox must have a positive width and height.")
				}
				scale := longest / math.Max(coords[2], coords[3])
				width, height = max(1, int(math.Round(coords[2]*scale))), max(1, int(math.Round(coords[3]*scale)))
				attrs := make([]xml.Attr, 0, len(node.Attr))
				for _, attr := range node.Attr {
					if attr.Name.Local != "width" && attr.Name.Local != "height" {
						attrs = append(attrs, attr)
					}
				}
				node.Attr = append(attrs, xml.Attr{Name: xml.Name{Local: "width"}, Value: strconv.FormatFloat(coords[2], 'f', -1, 64)}, xml.Attr{Name: xml.Name{Local: "height"}, Value: strconv.FormatFloat(coords[3], 'f', -1, 64)})
			}
			depth++
			token = node
		case xml.EndElement:
			if node.Name.Space == "" {
				node.Name.Space = "http://www.w3.org/2000/svg"
			}
			token = node
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return nil, 0, 0, errors.New("Choose a valid SVG file.")
			}
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, 0, 0, err
		}
	}
	if roots != 1 || depth != 0 {
		return nil, 0, 0, errors.New("Choose a valid SVG file.")
	}
	if err := encoder.Flush(); err != nil {
		return nil, 0, 0, err
	}
	return output.Bytes(), width, height, nil
}

func internalURLs(value string) bool {
	for {
		start := strings.Index(value, "url(")
		if start < 0 {
			return true
		}
		value = value[start+4:]
		end := strings.IndexByte(value, ')')
		if end < 0 {
			return false
		}
		target := strings.Trim(strings.TrimSpace(value[:end]), "\"'")
		if !strings.HasPrefix(target, "#") {
			return false
		}
		value = value[end+1:]
	}
}

func paintedBounds(img *image.NRGBA) image.Rectangle {
	bounds := image.Rectangle{}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.NRGBAAt(x, y).A != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return bounds
}

func singleColor(img *image.NRGBA) bool {
	// Use the most opaque pixel as the reference. An antialiased edge can
	// round to a different RGB value when unpremultiplied at low alpha.
	var reference color.NRGBA
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			p := img.NRGBAAt(x, y)
			if p.A > reference.A {
				reference = p
			}
		}
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			p := img.NRGBAAt(x, y)
			for _, difference := range []int{int(p.R) - int(reference.R), int(p.G) - int(reference.G), int(p.B) - int(reference.B)} {
				// Two premultiplied channel levels allow raster rounding,
				// while preserving meaningful color changes in opaque art.
				if abs(difference)*int(p.A) > 2*255 {
					return false
				}
			}
		}
	}
	return true
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func silhouette(img *image.NRGBA) *image.NRGBA {
	result := image.NewNRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			p := img.NRGBAAt(x, y)
			result.SetNRGBA(x, y, color.NRGBA{A: p.A})
		}
	}
	return result
}

// Fit painted bounds, rather than the source viewport. The default bounding
// diagonal stays inside the safe circle; explicit padding can extend beyond it.
func layer(img *image.NRGBA, size int, markDP float64) *image.NRGBA {
	bounds := paintedBounds(img)
	scale := markDP / 108 * float64(size) / math.Hypot(float64(bounds.Dx()), float64(bounds.Dy()))
	width, height := max(1, int(math.Floor(float64(bounds.Dx())*scale))), max(1, int(math.Floor(float64(bounds.Dy())*scale)))
	target := image.Rect((size-width)/2, (size-height)/2, (size-width)/2+width, (size-height)/2+height)
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(result, target, img, bounds, draw.Src, nil)
	return result
}

func validatePadding(padding *float64) error {
	if padding != nil && (math.IsNaN(*padding) || math.IsInf(*padding, 0) || *padding < -100 || *padding >= 50) {
		return errors.New("Choose padding from -100 up to (but not including) 50 percent with --padding.")
	}
	return nil
}

func paddingScale(padding float64) float64 {
	return 1 - 2*padding/100
}

func paddedMarkSize(img *image.NRGBA, padding float64) float64 {
	bounds := paintedBounds(img)
	// Convert an inset in the visible 72dp viewport to the layer's diagonal.
	return 72 * paddingScale(padding) * math.Hypot(float64(bounds.Dx()), float64(bounds.Dy())) / float64(max(bounds.Dx(), bounds.Dy()))
}
