package main

import (
	"context"
	_ "embed"
	"errors"
	"image"

	"github.com/tetratelabs/wazero"
)

// resvg compiled to WebAssembly from ./resvg, so SVG rendering needs no cgo or external renderer.
//
//go:generate sh resvg/build.sh
//go:embed resvg/resvg.wasm
var resvgWasm []byte

// renderSVG stretches the SVG to width×height.
func renderSVG(ctx context.Context, svg []byte, width, height int) (*image.RGBA, error) {
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)
	module, err := runtime.Instantiate(ctx, resvgWasm)
	if err != nil {
		return nil, err
	}
	memory := module.Memory()

	result, err := module.ExportedFunction("alloc").Call(ctx, uint64(len(svg)))
	if err != nil {
		return nil, err
	}
	input := uint32(result[0])
	if !memory.Write(input, svg) {
		return nil, errors.New("SVG does not fit in renderer memory")
	}
	result, err = module.ExportedFunction("render").Call(ctx, uint64(input), uint64(len(svg)), uint64(width), uint64(height))
	if err != nil {
		return nil, err
	}
	output := uint32(result[0])
	if output == 0 {
		return nil, errors.New("resvg could not render the SVG")
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	pixels, ok := memory.Read(output, uint32(len(img.Pix)))
	if !ok {
		return nil, errors.New("rendered pixels are outside renderer memory")
	}
	// resvg returns premultiplied RGBA, which is the image.RGBA layout.
	copy(img.Pix, pixels)
	return img, nil
}
