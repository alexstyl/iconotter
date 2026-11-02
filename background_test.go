package main

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	sideColor   = color.NRGBA{R: 200, G: 30, B: 30, A: 255}
	centerColor = color.NRGBA{R: 20, G: 90, B: 160, A: 255}
)

// backgroundFixture writes an opaque PNG whose center square is centerColor
// and whose excess along the longer side is sideColor.
func backgroundFixture(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	side := min(width, height)
	left, top := (width-side)/2, (height-side)/2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := sideColor
			if x >= left && x < left+side && y >= top && y < top+side {
				c = centerColor
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return writePNGFixture(t, img)
}

func writePNGFixture(t *testing.T, img image.Image) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "background.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertColor(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	for _, delta := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B), int(got.A) - int(want.A)} {
		if delta < -2 || delta > 2 {
			t.Fatalf("pixel (%d,%d) is %v, want %v", x, y, got, want)
		}
	}
}

func TestBackgroundImageCoversCanvasAndCropsCenter(t *testing.T) {
	for _, size := range [][2]int{{300, 100}, {100, 300}, {120, 120}} {
		bg, err := loadBackground(context.Background(), backgroundFixture(t, size[0], size[1]))
		if err != nil {
			t.Fatal(err)
		}
		canvas := bg.canvas(64)
		for _, p := range [][2]int{{0, 0}, {63, 0}, {0, 63}, {63, 63}, {32, 32}} {
			assertColor(t, canvas, p[0], p[1], centerColor)
		}
	}
}

func TestBackgroundAcceptsColorsAndRejectsUnusableImages(t *testing.T) {
	bg, err := loadBackground(context.Background(), "#123456")
	if err != nil || bg.image != nil || bg.color != (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}) {
		t.Fatalf("color background: %+v %v", bg, err)
	}
	translucent := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := range translucent.Pix {
		translucent.Pix[i] = 200
	}
	gif := filepath.Join(t.TempDir(), "pattern.gif")
	os.WriteFile(gif, []byte("GIF89a"), 0600)
	for value, want := range map[string]string{
		writePNGFixture(t, translucent): "transparent areas",
		gif:                             "--background FILE",
		filepath.Join(t.TempDir(), "missing.png"): "Could not open background image",
		"#12345": "#RRGGBB",
		fixture(t, `<svg viewBox="0 0 8 8"><rect width="4" height="8" fill="#000"/></svg>`): "transparent areas",
	} {
		if _, err := loadBackground(context.Background(), value); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v, want %q", value, err, want)
		}
	}
}

func TestSVGBackgroundCoversCanvas(t *testing.T) {
	bg, err := loadBackground(context.Background(), fixture(t, `<svg viewBox="0 0 10 10"><rect width="10" height="10" fill="#145AA0"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if bg.image.Bounds().Dx() != backgroundRenderSize {
		t.Fatalf("SVG background rendered at %v", bg.image.Bounds())
	}
	assertColor(t, bg.canvas(16), 0, 0, centerColor)
}

func TestImageBackgroundOnEveryPlatform(t *testing.T) {
	opts := defaults(t)
	opts.Type = "launcher"
	opts.Name = ""
	opts.Background = backgroundFixture(t, 300, 200)
	opts.BackgroundSet = true
	opts.MacOSStyle = "flat"
	opts.Platforms = []string{"android", "ios", "macos", "windows", "linux", "web"}
	root, err := generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	android := filepath.Join(root, "android", "res")
	if _, err := os.Stat(filepath.Join(android, "drawable", "ic_launcher_background.xml")); !os.IsNotExist(err) {
		t.Fatal("an image background must not also write the color drawable")
	}
	for _, d := range densities {
		layer := readPNG(t, filepath.Join(android, "drawable-"+d.name, "ic_launcher_background.png"))
		if layer.Bounds().Dx() != d.layer || layer.Bounds().Dy() != d.layer {
			t.Fatalf("%s background layer is %v", d.name, layer.Bounds())
		}
		assertColor(t, layer, 0, 0, centerColor)
	}
	adaptive, err := os.ReadFile(filepath.Join(android, "mipmap-anydpi-v26", "ic_launcher.xml"))
	if err != nil || !strings.Contains(string(adaptive), `android:drawable="@drawable/ic_launcher_background"`) {
		t.Fatalf("adaptive icon does not use the background layer: %s %v", adaptive, err)
	}
	guide, _ := os.ReadFile(filepath.Join(root, "android", "README.md"))
	if !strings.Contains(string(guide), "covers the visible 72dp") {
		t.Fatal("Android guide does not explain where the background image appears")
	}
	for _, check := range []struct {
		path string
		x, y int
	}{
		{"android/play-store.png", 0, 0},
		{"android/res/mipmap-xxxhdpi/ic_launcher.png", 96, 2},
		{"ios/Assets.xcassets/AppIcon.appiconset/icon-1024.png", 0, 0},
		{"macos/preview.png", 512, 110},
		{"windows/preview.png", 0, 0},
		{"linux/preview.png", 0, 0},
		{"web/apple-touch-icon.png", 0, 0},
	} {
		assertColor(t, readPNG(t, filepath.Join(root, check.path)), check.x, check.y, centerColor)
	}
	// The macOS tile keeps its transparent margin around the background.
	assertColor(t, readPNG(t, filepath.Join(root, "macos", "preview.png")), 40, 40, color.NRGBA{})
}

func TestGlassDocumentPutsImageBackgroundUnderArtwork(t *testing.T) {
	opts := defaults(t)
	opts.Name = "Studio"
	bg, err := loadBackground(context.Background(), backgroundFixture(t, 300, 200))
	if err != nil {
		t.Fatal(err)
	}
	artwork, err := loadArtwork(context.Background(), opts.Foreground)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := writeMacGlassDocument(context.Background(), opts, artwork, bg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(doc, "icon.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Fill   any `json:"fill"`
		Groups []struct {
			Layers []struct {
				Image string `json:"image-name"`
				Glass bool   `json:"glass"`
			} `json:"layers"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.Fill != "none" || len(document.Groups) != 2 || document.Groups[0].Layers[0].Image != "foreground.png" || document.Groups[1].Layers[0].Image != "background.png" || document.Groups[1].Layers[0].Glass {
		t.Fatalf("wrong glass document: %s", data)
	}
	layer := readPNG(t, filepath.Join(doc, "Assets", "background.png"))
	if layer.Bounds().Dx() != macCanvas || layer.Bounds().Dy() != macCanvas {
		t.Fatalf("background layer is %v", layer.Bounds())
	}
	assertColor(t, layer, 0, 0, centerColor)
}

func TestAppleGlassRendersImageBackground(t *testing.T) {
	if _, err := findIconComposerTool(); err != nil {
		t.Skip(err)
	}
	opts := defaults(t)
	opts.Name = "Studio"
	opts.Background = backgroundFixture(t, 300, 200)
	if err := writeMacOS(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	// Apple's renderer adds light glass edges; the middle of the tile beside the
	// artwork shows the background image.
	got := color.NRGBAModel.Convert(readPNG(t, filepath.Join(opts.Output, "preview.png")).At(512, 160)).(color.NRGBA)
	if got.B < got.R+80 {
		t.Fatalf("glass tile does not show the background image: %v", got)
	}
}

func TestAndroidBackgroundImageCoversVisibleAreaAndMirrorsMargin(t *testing.T) {
	// A white image with a red frame shows whether the image maps to the
	// visible 72dp of the layer rather than to the whole 108dp.
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			c := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			if x < 10 || x >= 90 || y < 10 || y >= 90 {
				c = sideColor
			}
			img.SetNRGBA(x, y, c)
		}
	}
	bg, err := loadBackground(context.Background(), writePNGFixture(t, img))
	if err != nil {
		t.Fatal(err)
	}
	layer := bg.adaptiveLayer(432)
	inset := 432 / 6
	for _, p := range [][2]int{{inset + 2, 216}, {432 - inset - 3, 216}, {216, inset + 2}, {216, 432 - inset - 3}} {
		assertColor(t, layer, p[0], p[1], sideColor)
	}
	assertColor(t, layer, 216, 216, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	// The hidden margin mirrors the frame instead of leaving it empty.
	assertColor(t, layer, inset-3, 216, sideColor)
	assertColor(t, layer, 0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
}
