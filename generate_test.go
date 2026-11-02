package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mark.svg")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120" viewBox="0 0 24 12"><rect x="5" y="3" width="14" height="6" rx="2" fill="#e85d04"/></svg>`

func defaults(t *testing.T) options {
	return options{Foreground: fixture(t, testSVG), Background: "#F0E8D0", Output: filepath.Join(t.TempDir(), "icons"), Name: "ic_launcher", MinSDK: 21, MarkSize: 56}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestBundleResourcesAndSafeArea(t *testing.T) {
	opts := defaults(t)
	path, err := generateAndroid(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range densities {
		for _, suffix := range []string{"foreground", "monochrome"} {
			img := readPNG(t, filepath.Join(path, "res", "drawable-"+d.name, opts.Name+"_"+suffix+".png"))
			if img.Bounds().Dx() != d.layer || img.Bounds().Dy() != d.layer {
				t.Fatalf("wrong %s layer size: %v", d.name, img.Bounds())
			}
			for y := 0; y < d.layer; y++ {
				for x := 0; x < d.layer; x++ {
					pixel := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					// Antialiasing adds up to one output pixel around fitted painted bounds.
					if pixel.A > 0 && math.Hypot(float64(x)+.5-float64(d.layer)/2, float64(y)+.5-float64(d.layer)/2) > 33*float64(d.layer)/108+1 {
						t.Fatalf("%s paint outside safe circle", d.name)
					}
					if suffix == "monochrome" && pixel.A > 0 && (pixel.R != 0 || pixel.G != 0 || pixel.B != 0) {
						t.Fatal("monochrome contains color")
					}
				}
			}
			if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
				t.Fatal("adaptive layer has an opaque corner")
			}
		}
		for _, suffix := range []string{"", "_round"} {
			img := readPNG(t, filepath.Join(path, "res", "mipmap-"+d.name, opts.Name+suffix+".png"))
			if img.Bounds().Dx() != d.legacy || img.Bounds().Dy() != d.legacy {
				t.Fatal("wrong legacy size")
			}
			if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
				t.Fatal("legacy corner is not masked")
			}
		}
	}
	for _, version := range []string{"26", "33"} {
		for _, suffix := range []string{"", "_round"} {
			body, err := os.ReadFile(filepath.Join(path, "res", "mipmap-anydpi-v"+version, opts.Name+suffix+".xml"))
			if err != nil {
				t.Fatal(err)
			}
			decoder := xml.NewDecoder(bytes.NewReader(body))
			mono := false
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if node, ok := tok.(xml.StartElement); ok {
					if node.Name.Local == "monochrome" {
						mono = true
					}
					for _, attr := range node.Attr {
						if attr.Name.Local == "drawable" && !strings.HasPrefix(attr.Value, "@drawable/ic_launcher_") {
							t.Fatalf("wrong resource reference: %s", attr.Value)
						}
					}
				}
			}
			if mono != (version == "33") {
				t.Fatal("wrong monochrome API range")
			}
		}
	}
	encodedStore, err := os.ReadFile(filepath.Join(path, "play-store.png"))
	if err != nil || len(encodedStore) < 26 || encodedStore[24] != 8 || encodedStore[25] != 6 {
		t.Fatal("Play Store PNG must use 8-bit RGBA channels (32-bit PNG)")
	}
	store := readPNG(t, filepath.Join(path, "play-store.png"))
	if store.Bounds() != image.Rect(0, 0, 512, 512) {
		t.Fatal("wrong Play Store dimensions")
	}
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			if _, _, _, alpha := store.At(x, y).RGBA(); alpha != 65535 {
				t.Fatal("Play Store icon is not opaque")
			}
		}
	}
	info, _ := os.Stat(filepath.Join(path, "play-store.png"))
	if info.Size() > 1024*1024 {
		t.Fatal("Play Store image exceeds size limit")
	}
	preview := readPNG(t, filepath.Join(path, "preview.png"))
	if preview.Bounds() != image.Rect(0, 0, 768, 512) {
		t.Fatal("missing previews")
	}
}

func TestPNGAndExplicitMonochrome(t *testing.T) {
	opts := defaults(t)
	mark := image.NewNRGBA(image.Rect(0, 0, 40, 60))
	for y := 10; y < 50; y++ {
		for x := 10; x < 30; x++ {
			p := color.NRGBA{R: 255, A: 255}
			if y > 30 {
				p = color.NRGBA{B: 255, A: 255}
			}
			mark.SetNRGBA(x, y, p)
		}
	}
	file := filepath.Join(t.TempDir(), "mark.png")
	var data bytes.Buffer
	png.Encode(&data, mark)
	os.WriteFile(file, data.Bytes(), 0600)
	opts.Foreground = file
	if _, err := generateAndroid(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "--monochrome") {
		t.Fatalf("multicolor artwork silently became a silhouette: %v", err)
	}
	if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
		t.Fatal("failed generation left output")
	}
	opts.Monochrome = fixture(t, testSVG)
	if _, err := generateAndroid(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
}

func TestExistingOutputAndInputsArePreserved(t *testing.T) {
	opts := defaults(t)
	before, _ := os.ReadFile(opts.Foreground)
	if _, err := generateAndroid(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(opts.Output, "keep.txt")
	os.WriteFile(marker, []byte("keep"), 0600)
	if _, err := generateAndroid(context.Background(), opts); err == nil {
		t.Fatal("existing destination accepted")
	}
	after, _ := os.ReadFile(opts.Foreground)
	preserved, _ := os.ReadFile(marker)
	if !bytes.Equal(before, after) || string(preserved) != "keep" {
		t.Fatal("input or output changed")
	}
}

func TestModernOnlyAndCustomName(t *testing.T) {
	opts := defaults(t)
	opts.MinSDK = 26
	opts.Name = "app_mark"
	path, err := generateAndroid(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "res", "mipmap-mdpi")); !os.IsNotExist(err) {
		t.Fatal("modern app contains legacy resources")
	}
	body, _ := os.ReadFile(filepath.Join(path, "manifest-snippet.xml"))
	if !bytes.Contains(body, []byte("@mipmap/app_mark_round")) {
		t.Fatal("round manifest reference is missing")
	}
	if _, err := os.Stat(filepath.Join(path, "res", "mipmap-anydpi-v26", "app_mark_round.xml")); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidOptionsAndArtwork(t *testing.T) {
	for _, change := range []func(*options){
		func(o *options) { o.Name = "../escape" }, func(o *options) { o.Name = "Invalid" }, func(o *options) { o.Background = "#FFFFFF00" },
		func(o *options) { o.MarkSize = 67 }, func(o *options) { o.MarkSize = math.NaN() }, func(o *options) { o.MarkSize = 47 }, func(o *options) { o.MinSDK = 0 },
		func(o *options) { o.Foreground = fixture(t, `<svg viewBox="0 0 24 24"/>`) },
		func(o *options) { o.Foreground = fixture(t, `<svg viewBox="0 0 24 24"><path`) },
	} {
		opts := defaults(t)
		change(&opts)
		if _, err := generateAndroid(context.Background(), opts); err == nil {
			t.Fatal("invalid request accepted")
		}
		if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
			t.Fatal("invalid request left output")
		}
	}
}

func TestSVGRejectsLinkedAssetsAndText(t *testing.T) {
	for _, svg := range []string{
		`<svg viewBox="0 0 24 24"><image href="file:///secret"/></svg>`,
		`<svg viewBox="0 0 24 24"><use href="other.svg#mark"/></svg>`,
		`<svg viewBox="0 0 24 24"><rect fill="url(https://example.com/a)"/></svg>`,
		`<svg viewBox="0 0 24 24"><text>Hi</text></svg>`,
		`<svg viewBox="0 0 24 24" onload="alert(1)"/>`,
		`<!DOCTYPE svg><svg viewBox="0 0 24 24"/>`,
		`<svg viewBox="0 0 NaN 24"/>`,
		`<svg viewBox="0 0 24 24"/><svg/>`,
	} {
		if _, _, _, err := normalizeSVG([]byte(svg)); err == nil {
			t.Fatalf("unsupported SVG accepted: %s", svg)
		}
	}
}

func TestSVGGradientAndInternalReferences(t *testing.T) {
	art := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><defs><linearGradient id="paint"><stop stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient><path id="mark" d="M4 4h16v16H4z"/></defs><use href="#mark" fill="url(#paint)"/></svg>`
	img, err := loadArtwork(context.Background(), fixture(t, art))
	if err != nil {
		t.Fatal(err)
	}
	if singleColor(img) {
		t.Fatal("SVG gradient or internal reference did not render")
	}
}

func TestCLIFlagsHelpAndPipedInput(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"launcher", "--help"}, {"notification", "--help"}} {
		var out, stderr bytes.Buffer
		if err := execute(context.Background(), args, strings.NewReader(""), &out, &stderr); err != nil || out.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("help failed: %v, %s", err, stderr.String())
		}
		if strings.Contains(out.String(), "completion") {
			t.Fatal("removed completion command is still advertised")
		}
	}
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"completion", "zsh"}, strings.NewReader(""), &out, &stderr); err == nil || !strings.Contains(stderr.String(), "unknown command") {
		t.Fatal("removed completion command was accepted")
	}
	out.Reset()
	stderr.Reset()
	if err := execute(context.Background(), []string{"launcher"}, strings.NewReader(""), &out, &stderr); err == nil || !strings.Contains(stderr.String(), "--foreground") || out.Len() != 0 {
		t.Fatal("piped command did not fail with actionable error")
	}
	opts := defaults(t)
	out.Reset()
	stderr.Reset()
	if err := execute(context.Background(), []string{"launcher", "--platform", "android", "--foreground", opts.Foreground, "--output", opts.Output, "--background", opts.Background}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Created launcher icons") || stderr.Len() != 0 {
		t.Fatalf("unexpected output: %s %s", out.String(), stderr.String())
	}
}

func TestAndroidResourceCompilation(t *testing.T) {
	if os.Getenv("ICONOTTER_TEST_ANDROID") != "1" {
		t.Skip("set ICONOTTER_TEST_ANDROID=1 and ANDROID_HOME to verify with aapt2")
	}
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		t.Fatal("set ANDROID_HOME to the Android SDK")
	}
	tools, _ := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "aapt2"))
	platform := filepath.Join(sdk, "platforms", "android-33", "android.jar")
	if len(tools) == 0 {
		t.Fatal("Android SDK has no aapt2")
	}
	if _, err := os.Stat(platform); err != nil {
		t.Fatal("install Android SDK platform 33 for this check")
	}
	for _, minimum := range []int{21, 26, 11} {
		opts := defaults(t)
		opts.MinSDK = minimum
		if minimum == 11 {
			opts.Type = "notification"
			opts.MarkSize = 22
		}
		path, err := generateAndroid(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		compiled := filepath.Join(t.TempDir(), "resources.zip")
		if output, err := exec.Command(tools[len(tools)-1], "compile", "--dir", filepath.Join(path, "res"), "-o", compiled).CombinedOutput(); err != nil {
			t.Fatalf("aapt2 compile: %v\n%s", err, output)
		}
		manifest := filepath.Join(t.TempDir(), "AndroidManifest.xml")
		application := `<application android:icon="@mipmap/ic_launcher" android:roundIcon="@mipmap/ic_launcher_round"/>`
		if opts.Type == "notification" {
			application = `<application><meta-data android:name="notification-icon" android:resource="@drawable/ic_launcher"/></application>`
		}
		os.WriteFile(manifest, []byte(fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.icon"><uses-sdk android:minSdkVersion="%d"/>%s</manifest>`, minimum, application)), 0600)

		if output, err := exec.Command(tools[len(tools)-1], "link", "-I", platform, "--manifest", manifest, "-o", filepath.Join(t.TempDir(), "icons.apk"), compiled).CombinedOutput(); err != nil {
			t.Fatalf("aapt2 link: %v\n%s", err, output)
		}
	}
}

func TestAntialiasedSingleColorArtwork(t *testing.T) {
	img, err := loadArtwork(context.Background(), fixture(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 2L22 10V22H15V15H9V22H2V10Z" fill="#164E3C"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if !singleColor(img) {
		t.Fatal("antialiasing was mistaken for multiple colors")
	}
}
