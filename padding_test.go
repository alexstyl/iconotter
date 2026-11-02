package main

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLauncherPaddingComposition(t *testing.T) {
	// Distinct top/bottom bands expose incorrect clipping or stretching.
	const art = `<svg viewBox="0 0 80 80"><rect width="80" height="80" fill="#ff0000"/><rect y="10" width="80" height="60" fill="#0000ff"/></svg>`
	for _, padding := range []string{"20", "0", "-25"} {
		t.Run(padding, func(t *testing.T) {
			opts := defaults(t)
			var out, stderr bytes.Buffer
			err := execute(context.Background(), []string{"launcher", "--foreground", fixture(t, art), "--monochrome", fixture(t, `<svg viewBox="0 0 80 80"><rect width="80" height="80" fill="#000000"/></svg>`), "--background", "#FFFFFF", "--padding=" + padding, "--output", opts.Output}, strings.NewReader(""), &out, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			// Compare the same visible locations across native adaptive, store,
			// legacy, themed, and iOS output, allowing for density rounding.
			for _, path := range []string{
				"android/res/drawable-xxxhdpi/ic_launcher_foreground.png",
				"android/res/drawable-xxxhdpi/ic_launcher_monochrome.png",
				"android/play-store.png",
				"android/res/mipmap-xxxhdpi/ic_launcher.png",
				"ios/Assets.xcassets/AppIcon.appiconset/icon-1024.png",
			} {
				img := readPNG(t, filepath.Join(opts.Output, path))
				size, inset := img.Bounds().Dx(), 0
				adaptive := strings.Contains(path, "drawable-")
				if adaptive {
					inset = size / 6
				}
				visible := size - 2*inset
				x, y := size/2, inset+visible/20
				p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if padding == "20" {
					if adaptive && p.A != 0 || !adaptive && p != (color.NRGBA{255, 255, 255, 255}) {
						t.Fatalf("%s: positive padding didn't leave space: %v", path, p)
					}
				} else if strings.Contains(path, "monochrome") {
					if p.A != 255 {
						t.Fatalf("%s: silhouette does not reach visible edge", path)
					}
				} else {
					want := color.NRGBA{255, 0, 0, 255}
					if padding == "-25" {
						want = color.NRGBA{0, 0, 255, 255}
					}
					if p != want {
						t.Fatalf("%s: expected centered crop %v, got %v", path, want, p)
					}
				}
			}
		})
	}
}

func TestInvalidPaddingDoesNotPublish(t *testing.T) {
	for _, args := range [][]string{
		{"launcher", "--padding=NaN"},
		{"launcher", "--padding=+Inf"},
		{"launcher", "--padding=-101"},
		{"launcher", "--padding=50"},
		{"launcher", "--padding=0", "--size=66"},
		{"launcher", "--platform=ios", "--padding=50"},
		{"notification", "--padding=0"},
	} {
		opts := defaults(t)
		args = append(args, "--foreground", opts.Foreground, "--output", opts.Output)
		var out, stderr bytes.Buffer
		if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err == nil {
			t.Fatalf("accepted invalid padding: %v", args)
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("invalid padding left output")
		}
	}
}
