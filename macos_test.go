package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMacOSExportsEveryRepresentationAndNativeICNS(t *testing.T) {
	opts := defaults(t)
	opts.MacOSStyle = "flat"
	opts.Type = "launcher"
	opts.Platforms = []string{"macos"}
	opts.Name = "Studio"
	output, err := generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(output, "macos")
	data, err := os.ReadFile(filepath.Join(root, "Studio.icns"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data) {
		t.Fatal("invalid ICNS header")
	}
	chunks := map[string][]byte{}
	for cursor := 8; cursor < len(data); {
		if cursor+8 > len(data) {
			t.Fatal("truncated chunk header")
		}
		length := int(binary.BigEndian.Uint32(data[cursor+4 : cursor+8]))
		if length < 8 || cursor+length > len(data) {
			t.Fatal("invalid chunk length")
		}
		chunks[string(data[cursor:cursor+4])] = data[cursor+8 : cursor+length]
		cursor += length
	}
	contents, err := os.ReadFile(filepath.Join(root, "Assets.xcassets/Studio.appiconset/Contents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Images []struct{ Filename, Idiom, Size, Scale string }
	}
	if err := json.Unmarshal(contents, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Images) != 10 || len(chunks) != 10 {
		t.Fatal("missing icon representations")
	}
	for index, slot := range macIconSlots {
		image, err := png.Decode(bytes.NewReader(chunks[slot.code]))
		if err != nil {
			t.Fatal(err)
		}
		if image.Bounds().Dx() != slot.points*slot.scale || image.Bounds().Dy() != slot.points*slot.scale {
			t.Fatalf("wrong dimensions for %s", slot.code)
		}
		entry := catalog.Images[index]
		asset, err := os.ReadFile(filepath.Join(root, "Assets.xcassets/Studio.appiconset", entry.Filename))
		if err != nil {
			t.Fatal(err)
		}
		iconset, err := os.ReadFile(filepath.Join(root, "Studio.iconset", entry.Filename))
		if err != nil {
			t.Fatal(err)
		}
		if entry.Idiom != "mac" || !bytes.Equal(asset, chunks[slot.code]) || !bytes.Equal(asset, iconset) {
			t.Fatal("ICNS, iconset and asset catalog differ")
		}
	}
	master, err := loadArtwork(context.Background(), filepath.Join(root, "preview.png"))
	if err != nil {
		t.Fatal(err)
	}
	if bounds := paintedBounds(master); bounds.Min.X != 100 || bounds.Min.Y != 100 || bounds.Max.X != 924 || bounds.Max.Y != 924 {
		t.Fatalf("incorrect canvas margin: %v", bounds)
	}
	if color.NRGBAModel.Convert(master.At(100, 100)).(color.NRGBA).A != 0 {
		t.Fatal("outer tile corner is not transparent")
	}
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("iconutil"); err == nil {
			decoded := filepath.Join(t.TempDir(), "native.iconset")
			if body, err := exec.Command("iconutil", "--convert", "iconset", filepath.Join(root, "Studio.icns"), "--output", decoded).CombinedOutput(); err != nil {
				t.Fatalf("macOS rejected ICNS: %v\n%s", err, body)
			}
			native := readPNG(t, filepath.Join(decoded, "icon_512x512@2x.png"))
			if native.Bounds() != master.Bounds() {
				t.Fatal("native ICNS decoding lost the largest representation")
			}
		}
	}
}

func TestMacOSContentPaddingNeverRemovesCanvasMargin(t *testing.T) {
	art, err := loadArtwork(context.Background(), fixture(t, `<svg viewBox="0 0 80 40"><rect width="80" height="40" fill="#ff0000"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, padding := range []float64{0, 20, -25} {
		icon := composeMacIcon(art, color.NRGBA{255, 255, 255, 255}, &padding)
		if bounds := paintedBounds(icon); bounds.Min.X != 100 || bounds.Max.X != 924 {
			t.Fatalf("padding %g changed outer margin: %v", padding, bounds)
		}
		// Center stays colored; aspect ratio is preserved and the tile clips overscan.
		center := color.NRGBAModel.Convert(icon.At(512, 512)).(color.NRGBA)
		if center.R != 255 || center.G != 0 || center.A != 255 {
			t.Fatalf("incorrect center: %v", center)
		}
		edge := color.NRGBAModel.Convert(icon.At(120, 512)).(color.NRGBA)
		if padding == 20 && edge.G != 255 {
			t.Fatal("positive padding did not inset artwork")
		}
		if padding <= 0 && edge.G != 0 {
			t.Fatal("zero/negative padding did not fill horizontal bounds")
		}
	}
}

func TestMacOSCommandValidationAndInstructions(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	args := []string{"launcher", "--platform", "macos", "--macos-style", "flat", "--foreground", opts.Foreground, "--output", opts.Output}
	if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "macos/AppIcon.icns") || strings.Contains(out.String(), "AndroidManifest") {
		t.Fatalf("incorrect instructions: %s", out.String())
	}
	if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err == nil {
		t.Fatal("overwrote existing output")
	}
	for _, flag := range []string{"--size=56", "--min-sdk=21", "--monochrome=" + opts.Foreground} {
		err := execute(context.Background(), append(args, flag), strings.NewReader(""), &out, &stderr)
		if err == nil || !strings.Contains(err.Error(), "applies to Android") {
			t.Fatalf("accepted Android-only option %s: %v", flag, err)
		}
	}
}
