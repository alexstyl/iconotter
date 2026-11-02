package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusBarResources(t *testing.T) {
	opts := defaults(t)
	opts.Type = "notification"
	opts.Name = "ic_stat_message"
	opts.MarkSize = 22
	path, err := generateAndroid(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for bucket, size := range map[string]int{"ldpi": 18, "mdpi": 24, "hdpi": 36, "xhdpi": 48, "xxhdpi": 72, "xxxhdpi": 96} {
		file := filepath.Join(path, "res", "drawable-"+bucket, "ic_stat_message.png")
		img := readPNG(t, file)
		if img.Bounds() != image.Rect(0, 0, size, size) {
			t.Fatalf("wrong %s status-bar size: %v", bucket, img.Bounds())
		}
		data, _ := os.ReadFile(file)
		if len(data) < 26 || data[24] != 8 || data[25] != 6 {
			t.Fatal("status-bar icon is not 32-bit PNG")
		}
		painted := false
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if p.A > 0 {
					painted = true
					if p.R != 255 || p.G != 255 || p.B != 255 {
						t.Fatalf("status-bar icon contains colored pixels: %v", p)
					}
				}
			}
		}
		if !painted {
			t.Fatal("status-bar icon is blank")
		}
		if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
			t.Fatal("status-bar icon has an opaque background")
		}
	}
	if _, err := os.Stat(filepath.Join(path, "manifest-snippet.xml")); !os.IsNotExist(err) {
		t.Fatal("status-bar export suggests launcher manifest changes")
	}
	if _, err := os.Stat(filepath.Join(path, "play-store.png")); !os.IsNotExist(err) {
		t.Fatal("status-bar export contains launcher resources")
	}
}

func TestStatusBarCLIAndValidation(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	args := []string{"notification", "--platform", "android", "--foreground", opts.Foreground, "--output", opts.Output}
	if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), ".setSmallIcon(R.drawable.ic_stat_notification)") || strings.Contains(out.String(), "AndroidManifest") || stderr.Len() != 0 {
		t.Fatalf("wrong instructions: %s %s", out.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(opts.Output, "android", "res", "drawable-mdpi", "ic_stat_notification.png")); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--background", "#FFFFFF"}, {"--min-sdk", "10"}, {"--size", "25"}, {"--platform", "other"}} {
		opts := defaults(t)
		out.Reset()
		stderr.Reset()
		args := append([]string{"notification", "--platform", "android", "--foreground", opts.Foreground, "--output", opts.Output}, flags...)
		if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err == nil || out.Len() != 0 {
			t.Fatalf("invalid status-bar flags accepted: %v", flags)
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("invalid options left output")
		}
	}
}

func TestStatusBarRejectsOpaqueAndMulticolorArtwork(t *testing.T) {
	for _, art := range []string{
		`<svg viewBox="0 0 24 24"><rect width="24" height="24" fill="red"/></svg>`,
		`<svg viewBox="0 0 24 24"><rect x="2" y="2" width="10" height="10" fill="red"/><rect x="12" y="12" width="10" height="10" fill="blue"/></svg>`,
	} {
		opts := defaults(t)
		opts.Type = "notification"
		opts.MarkSize = 22
		opts.Foreground = fixture(t, art)
		if _, err := generateAndroid(context.Background(), opts); err == nil {
			t.Fatal("unsuitable notification artwork accepted")
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("rejected artwork left output")
		}
		opts.Monochrome = fixture(t, testSVG)
		if _, err := generateAndroid(context.Background(), opts); err != nil {
			t.Fatal(err)
		}
	}
}
