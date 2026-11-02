package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlatformSelection(t *testing.T) {
	for _, test := range []struct {
		kind        string
		input, want []string
	}{
		{"launcher", []string{"android", "ios"}, []string{"android", "ios"}},
		{"launcher", []string{"ios", "android", "ios"}, []string{"ios", "android"}},
		{"launcher", []string{" IOS "}, []string{"ios"}},
		{"launcher", []string{"macos", "ios", "macos"}, []string{"macos", "ios"}},
		{"launcher", []string{"web", "ios", "web"}, []string{"web", "ios"}},
		{"launcher", []string{"windows", "linux", "windows"}, []string{"windows", "linux"}},
		{"notification", []string{"android"}, []string{"android"}},
	} {
		got, err := selectPlatforms(test.kind, test.input)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("got %v, %v", got, err)
		}
	}
	for _, test := range []struct {
		kind  string
		input []string
	}{
		{"notification", []string{"ios"}}, {"launcher", nil}, {"launcher", []string{"android", ""}},
		{"launcher", []string{"unknown"}}, {"notification", []string{"macos"}}, {"notification", []string{"web"}}, {"notification", []string{"linux"}}, {"other", []string{"android"}},
	} {
		if _, err := selectPlatforms(test.kind, test.input); err == nil {
			t.Fatalf("accepted unsupported platforms: %v", test)
		}
	}
}

func TestLauncherExportsBothPlatformsByDefault(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", opts.Foreground, "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"android/res/mipmap-anydpi-v26/ic_launcher.xml",
		"android/res/mipmap-anydpi-v33/ic_launcher_round.xml",
		"ios/Assets.xcassets/AppIcon.appiconset/Contents.json",
		"android/preview.png", "ios/preview.png",
	} {
		if _, err := os.Stat(filepath.Join(opts.Output, path)); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(out.String(), "AppIcon") || !strings.Contains(out.String(), "@mipmap/ic_launcher") || stderr.Len() != 0 {
		t.Fatalf("wrong multi-platform instructions: %s %s", out.String(), stderr.String())
	}
}

func TestIOSAssetCatalogAndOpaqueJPEG(t *testing.T) {
	opts := defaults(t)
	source := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 220, B: 30, A: 255})
		}
	}
	file := filepath.Join(t.TempDir(), "source.jpg")
	var data bytes.Buffer
	if err := jpeg.Encode(&data, source, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", file, "--platform", "ios", "--background", "#FFFFFF", "--name", "MangaIcon", "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(opts.Output, "ios", "Assets.xcassets", "MangaIcon.appiconset")
	contents, err := os.ReadFile(filepath.Join(set, "Contents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Images []struct{ Filename, Idiom, Platform, Size string }
		Info   struct{ Version int }
	}
	if err := json.Unmarshal(contents, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Images) != 1 || catalog.Images[0].Size != "1024x1024" || catalog.Images[0].Idiom != "universal" || catalog.Images[0].Platform != "ios" || catalog.Info.Version != 1 {
		t.Fatalf("invalid iOS icon catalog: %s", contents)
	}
	img := readPNG(t, filepath.Join(set, catalog.Images[0].Filename))
	if img.Bounds() != image.Rect(0, 0, 1024, 1024) {
		t.Fatal("wrong iOS image size")
	}
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 65535 {
				t.Fatal("iOS icon contains transparency")
			}
		}
	}
	encoded, _ := os.ReadFile(filepath.Join(set, catalog.Images[0].Filename))
	if encoded[24] != 8 || encoded[25] != 2 {
		t.Fatal("iOS icon must be an RGB PNG without an alpha channel")
	}
	corner := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if corner.R != 255 || corner.G != 255 || corner.B != 255 {
		t.Fatal("iOS background or corner mask is wrong")
	}
	if _, err := os.Stat(filepath.Join(opts.Output, "android")); !os.IsNotExist(err) {
		t.Fatal("iOS-only export contains Android files")
	}
	if strings.Contains(out.String(), "compileSdk") {
		t.Fatal("Android instructions leaked into iOS-only export")
	}
}

func TestInvalidPlatformCombinationsDoNotCreateOutput(t *testing.T) {
	for _, args := range [][]string{
		{"notification", "--platform", "ios"},
		{"notification", "--platform", "android,ios"},
		{"launcher", "--platform", "unknown"},
		{"launcher", "--platform", "ios", "--min-sdk", "21"},
		{"launcher", "--platform", "ios", "--size", "56"},
		{"launcher", "--platform", "ios", "--monochrome", "ignored.svg"},
	} {
		opts := defaults(t)
		args = append(args, "--foreground", opts.Foreground, "--output", opts.Output)
		var out, stderr bytes.Buffer
		if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err == nil || out.Len() != 0 {
			t.Fatalf("invalid options accepted: %v", args)
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("invalid combination left files")
		}
	}
}

func TestMultiplePlatformFailureIsAtomic(t *testing.T) {
	opts := defaults(t)
	opts.Type = "launcher"
	opts.Platforms = []string{"ios", "android"}
	opts.Name = "AppIcon"
	// Valid for iOS, invalid for Android: the completed iOS export must roll back.
	if _, err := generate(context.Background(), opts); err == nil {
		t.Fatal("invalid Android name accepted")
	}
	if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
		t.Fatal("partial multi-platform export was published")
	}
	stages, _ := filepath.Glob(filepath.Join(filepath.Dir(opts.Output), ".iconotter-*"))
	if len(stages) != 0 {
		t.Fatal("failed export left staging files")
	}
}
