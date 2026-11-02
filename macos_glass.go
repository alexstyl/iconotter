package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	xdraw "golang.org/x/image/draw"
)

func findIconComposerTool() (string, error) {
	if runtime.GOOS == "darwin" {
		candidates := []string{"/Applications/Icon Composer.app/Contents/Executables/ictool", "/Applications/Xcode.app/Contents/Applications/Icon Composer.app/Contents/Executables/ictool"}
		if developer, err := exec.Command("xcode-select", "-p").Output(); err == nil {
			candidates = append(candidates, filepath.Join(strings.TrimSpace(string(developer)), "../Applications/Icon Composer.app/Contents/Executables/ictool"))
		}
		if tool, err := exec.LookPath("ictool"); err == nil {
			candidates = append(candidates, tool)
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return candidate, nil
			}
		}
	}
	return "", errors.New("Glass icons require Apple's Icon Composer on macOS. Install Icon Composer or choose --macos-style flat.")
}

// Apple renders the background. The artwork layer is deliberately opaque/flat
// in material terms, while preserving the artwork's own alpha channel. A color
// becomes the document's automatic-gradient fill. An image becomes a full-bleed
// layer in the bottom group, because the first group is drawn on top. That
// group multiplies over Apple's automatic gradient of white, so Icon Composer
// still renders the lighter top and darker bottom. Icon Composer ignores
// blend modes on layers; it applies them to groups.
func macGlassDocument(name string, background iconBackground) map[string]any {
	layer := func(layerName, imageName string) map[string]any {
		return map[string]any{"name": layerName, "image-name": imageName, "glass": false, "position": map[string]any{"scale": 1, "translation-in-points": []int{0, 0}}}
	}
	group := func(shadow string, layers ...any) map[string]any {
		return map[string]any{
			"layers":       layers,
			"shadow":       map[string]any{"kind": shadow, "opacity": 0},
			"translucency": map[string]any{"enabled": false, "value": 0},
		}
	}
	groups := []any{group("neutral", layer(name, "foreground.png"))}
	base := background.color
	if background.image != nil {
		base = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		backgroundGroup := group("none", layer("Background", "background.png"))
		backgroundGroup["blend-mode"] = "multiply"
		groups = append(groups, backgroundGroup)
	}
	fill := map[string]any{"automatic-gradient": fmt.Sprintf("srgb:%.5f,%.5f,%.5f,1.00000", float64(base.R)/255, float64(base.G)/255, float64(base.B)/255)}
	return map[string]any{"fill": fill, "groups": groups, "supported-platforms": map[string]any{"squares": "shared"}}
}

func writeMacGlassDocument(ctx context.Context, opts options, artwork *image.NRGBA, background iconBackground) (string, error) {
	doc := opts.Name + ".icon"
	layer := image.NewNRGBA(image.Rect(0, 0, macCanvas, macCanvas))
	bounds := paintedBounds(artwork)
	fraction := .75
	if opts.Padding != nil {
		fraction = paddingScale(*opts.Padding)
	}
	scale := macCanvas * fraction / float64(max(bounds.Dx(), bounds.Dy()))
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	target := image.Rect((macCanvas-width)/2, (macCanvas-height)/2, (macCanvas-width)/2+width, (macCanvas-height)/2+height)
	xdraw.CatmullRom.Scale(layer, target, artwork, bounds, draw.Over, nil)
	if err := writePNGFile(ctx, opts.Output, filepath.Join(doc, "Assets/foreground.png"), layer); err != nil {
		return "", err
	}
	if background.image != nil {
		if err := writePNGFile(ctx, opts.Output, filepath.Join(doc, "Assets/background.png"), background.canvas(macCanvas)); err != nil {
			return "", err
		}
	}
	data, err := json.MarshalIndent(macGlassDocument(opts.Name, background), "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeBundleFile(ctx, opts.Output, filepath.Join(doc, "icon.json"), append(data, '\n')); err != nil {
		return "", err
	}
	return filepath.Join(opts.Output, doc), nil
}

type macGlassRenderer struct {
	tool, document, scratch string
	cache                   map[int]*image.NRGBA
}

func (renderer *macGlassRenderer) render(ctx context.Context, size int) (*image.NRGBA, error) {
	if cached := renderer.cache[size]; cached != nil {
		return cached, nil
	}
	body := max(1, int(math.Round(float64(size)*macTile/macCanvas)))
	path := filepath.Join(renderer.scratch, fmt.Sprintf("render-%d.png", size))
	cmd := exec.CommandContext(ctx, renderer.tool, renderer.document, "--export-image", "--output-file", path, "--platform", "macOS", "--rendition", "Default", "--width", fmt.Sprint(body), "--height", fmt.Sprint(body), "--scale", "1")
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Icon Composer could not render the macOS icon: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	// ictool writes Display P3. Go's PNG decoder does not interpret ICC profiles,
	// so use Apple's ColorSync conversion before reading and re-encoding pixels.
	convert := exec.CommandContext(ctx, "/usr/bin/sips", "--matchTo", "/System/Library/ColorSync/Profiles/sRGB Profile.icc", path)
	if output, err := convert.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Could not convert Icon Composer's output to sRGB: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	rendered, err := loadArtwork(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("Could not read Icon Composer's output: %w", err)
	}
	if rendered.Bounds().Dx() != body || rendered.Bounds().Dy() != body {
		return nil, errors.New("Icon Composer returned an unexpected image size.")
	}
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	inset := (size - body) / 2
	draw.Draw(icon, image.Rect(inset, inset, inset+body, inset+body), rendered, rendered.Bounds().Min, draw.Src)
	renderer.cache[size] = icon
	return icon, nil
}
