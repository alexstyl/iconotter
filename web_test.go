package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebExport(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", opts.Foreground, "--background", "#F0E8D0", "--platform", "web", "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 || !strings.Contains(out.String(), "web/favicon.ico") {
		t.Fatalf("missing web integration instructions: %s %s", out.String(), stderr.String())
	}
	for _, test := range []struct {
		file string
		size int
	}{
		{"favicon-16x16.png", 16}, {"favicon-32x32.png", 32},
		{"apple-touch-icon.png", 180}, {"preview.png", 256},
	} {
		icon := readPNG(t, filepath.Join(opts.Output, "web", test.file))
		if icon.Bounds() != image.Rect(0, 0, test.size, test.size) {
			t.Fatalf("%s has wrong size: %v", test.file, icon.Bounds())
		}
		if got := color.NRGBAModel.Convert(icon.At(0, 0)).(color.NRGBA); got != (color.NRGBA{R: 240, G: 232, B: 208, A: 255}) {
			t.Fatalf("%s has wrong background: %v", test.file, got)
		}
	}
	ico, err := os.ReadFile(filepath.Join(opts.Output, "web", "favicon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ico) < 54 || binary.LittleEndian.Uint16(ico[2:4]) != 1 || binary.LittleEndian.Uint16(ico[4:6]) != 3 {
		t.Fatal("invalid ICO header")
	}
	for i, size := range []int{16, 32, 48} {
		entry := ico[6+i*16 : 6+(i+1)*16]
		length := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if int(entry[0]) != size || int(entry[1]) != size || offset+length > len(ico) ||
			binary.LittleEndian.Uint32(ico[offset:offset+4]) != 40 ||
			binary.LittleEndian.Uint32(ico[offset+4:offset+8]) != uint32(size) ||
			binary.LittleEndian.Uint32(ico[offset+8:offset+12]) != uint32(size*2) {
			t.Fatalf("invalid %dpx ICO entry", size)
		}
	}
	if _, err := os.Stat(filepath.Join(opts.Output, "ios")); !os.IsNotExist(err) {
		t.Fatal("web-only export contains iOS files")
	}
}

func TestWebExportFailureLeavesNoBundle(t *testing.T) {
	opts := defaults(t)
	var out, stderr bytes.Buffer
	if err := execute(context.Background(), []string{"launcher", "--foreground", opts.Foreground, "--background", "invalid", "--platform", "web", "--output", opts.Output}, strings.NewReader(""), &out, &stderr); err == nil {
		t.Fatal("accepted invalid background")
	}
	if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
		t.Fatal("failed web export left output")
	}
}
