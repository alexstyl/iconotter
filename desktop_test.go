package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWindowsAndLinuxExports(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", opts.Foreground, "--platform", "windows,linux", "--name", "sample-app", "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 || !strings.Contains(out.String(), "windows/sample-app.ico") || !strings.Contains(out.String(), "linux/sample-app.png") {
		t.Fatalf("missing desktop instructions: %s %s", out.String(), stderr.String())
	}
	ico, err := os.ReadFile(filepath.Join(opts.Output, "windows", "sample-app.ico"))
	if err != nil {
		t.Fatal(err)
	}
	sizes := []int{16, 24, 32, 48, 64, 96, 256}
	if binary.LittleEndian.Uint16(ico[4:6]) != uint16(len(sizes)) {
		t.Fatal("wrong number of Windows icon sizes")
	}
	for i, size := range sizes {
		entry := ico[6+i*16 : 6+(i+1)*16]
		length := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if int(entry[0]) != size%256 || int(entry[1]) != size%256 || offset+length > len(ico) {
			t.Fatalf("invalid Windows icon entry for %dpx", size)
		}
		if size == 256 {
			if !bytes.Equal(ico[offset:offset+8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
				t.Fatal("256px Windows icon must be PNG-compressed")
			}
		} else if binary.LittleEndian.Uint32(ico[offset:offset+4]) != 40 {
			t.Fatalf("%dpx Windows icon is not BMP data", size)
		}
		if size == 16 {
			maskStart := offset + 40 + size*size*4
			if ico[maskStart]&0x80 == 0 {
				t.Fatal("transparent Windows icon has no AND mask")
			}
		}
	}
	for _, size := range []int{16, 24, 32, 48, 64, 128, 256, 512} {
		dir := strconv.Itoa(size) + "x" + strconv.Itoa(size)
		icon := readPNG(t, filepath.Join(opts.Output, "linux", "hicolor", dir, "apps", "sample-app.png"))
		if icon.Bounds() != image.Rect(0, 0, size, size) {
			t.Fatalf("wrong Linux %dpx icon size", size)
		}
		if got := color.NRGBAModel.Convert(icon.At(0, 0)).(color.NRGBA); got.A != 0 {
			t.Fatalf("Linux icon lost source transparency: %v", got)
		}
	}
	if icon := readPNG(t, filepath.Join(opts.Output, "linux", "sample-app.png")); icon.Bounds() != image.Rect(0, 0, 512, 512) {
		t.Fatal("wrong Compose Linux icon")
	}
}

func TestDesktopBackgroundIsOptIn(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", opts.Foreground, "--platform", "linux", "--background", "#123456", "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	icon := readPNG(t, filepath.Join(opts.Output, "linux", "app-icon.png"))
	if got := color.NRGBAModel.Convert(icon.At(0, 0)).(color.NRGBA); got != (color.NRGBA{R: 18, G: 52, B: 86, A: 255}) {
		t.Fatalf("explicit background was ignored: %v", got)
	}
}
