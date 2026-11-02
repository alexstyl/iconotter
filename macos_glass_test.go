package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlassDocumentKeepsForegroundFlatAndPreservesPadding(t *testing.T) {
	opts := defaults(t)
	opts.Name = "Studio"
	opts.Padding = new(float64)
	*opts.Padding = 16
	artwork, err := loadArtwork(context.Background(), opts.Foreground)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := writeMacGlassDocument(context.Background(), opts, artwork, iconBackground{color: color.NRGBA{245, 245, 247, 255}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(doc, "icon.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatForeground(t, data)
	layer, err := loadArtwork(context.Background(), filepath.Join(doc, "Assets/foreground.png"))
	if err != nil {
		t.Fatal(err)
	}
	bounds := paintedBounds(layer)
	if max(bounds.Dx(), bounds.Dy()) != 696 {
		t.Fatalf("foreground padding was not normalized: %v", bounds)
	}
	if alpha := color.NRGBAModel.Convert(layer.At(0, 0)).(color.NRGBA).A; alpha != 0 {
		t.Fatal("foreground has an opaque backdrop")
	}
	if layer.NRGBAAt(512, 512) != artwork.NRGBAAt(artwork.Bounds().Dx()/2, artwork.Bounds().Dy()/2) {
		t.Fatal("foreground color changed")
	}
}

func assertFlatForeground(t *testing.T, data []byte) {
	t.Helper()
	var doc struct {
		Fill   map[string]string `json:"fill"`
		Groups []struct {
			Layers []struct {
				Glass bool   `json:"glass"`
				Image string `json:"image-name"`
			} `json:"layers"`
			Shadow struct {
				Opacity float64 `json:"opacity"`
			} `json:"shadow"`
			Translucency struct {
				Enabled bool    `json:"enabled"`
				Value   float64 `json:"value"`
			} `json:"translucency"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Fill["automatic-gradient"] == "" || len(doc.Groups) != 1 || len(doc.Groups[0].Layers) != 1 {
		t.Fatal("missing background or foreground")
	}
	group := doc.Groups[0]
	if group.Layers[0].Glass || group.Layers[0].Image != "foreground.png" || group.Shadow.Opacity != 0 || group.Translucency.Enabled || group.Translucency.Value != 0 {
		t.Fatal("glass effects enabled on foreground")
	}
}

func TestMacOSDefaultsToRealAppleGlassRendering(t *testing.T) {
	if _, err := findIconComposerTool(); err != nil {
		t.Skip(err)
	}
	opts := defaults(t)
	var out, stderr bytes.Buffer
	err := execute(context.Background(), []string{"launcher", "--platform", "macos", "--foreground", opts.Foreground, "--name", "Studio", "--output", opts.Output}, strings.NewReader(""), &out, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(opts.Output, "macos")
	document, err := os.ReadFile(filepath.Join(root, "Studio.icon/icon.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatForeground(t, document)
	icon := readPNG(t, filepath.Join(root, "preview.png"))
	if icon.Bounds().Dx() != 1024 || icon.Bounds().Dy() != 1024 {
		t.Fatal("invalid native rendering size")
	}
	// The foreground's own color remains intact after Apple's renderer runs.
	original, err := loadArtwork(context.Background(), opts.Foreground)
	if err != nil {
		t.Fatal(err)
	}
	want := original.NRGBAAt(original.Bounds().Dx()/2, original.Bounds().Dy()/2)
	got := color.NRGBAModel.Convert(icon.At(512, 512)).(color.NRGBA)
	for _, delta := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B)} {
		if delta < -2 || delta > 2 {
			t.Fatalf("foreground was tinted: got %v want %v", got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Studio.icns")); err != nil {
		t.Fatal(err)
	}
}

func TestGlassErrorsNeverFallBackToSimulatedGlass(t *testing.T) {
	renderer := macGlassRenderer{tool: filepath.Join(t.TempDir(), "missing-ictool"), scratch: t.TempDir()}
	if icon, err := renderer.render(context.Background(), 32); err == nil || icon != nil {
		t.Fatal("missing renderer silently produced an icon")
	}
	opts := defaults(t)
	for _, args := range [][]string{
		{"launcher", "--platform", "ios", "--macos-style", "glass", "--foreground", opts.Foreground, "--output", opts.Output},
		{"launcher", "--platform", "macos", "--macos-style", "imaginary", "--foreground", opts.Foreground, "--output", opts.Output},
	} {
		var out, stderr bytes.Buffer
		if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err == nil {
			t.Fatal("invalid style selection was accepted")
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("failed export published partial files")
		}
	}
}
