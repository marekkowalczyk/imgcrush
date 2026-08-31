package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/delthas/octreequant"
)

func encodePNGTruecolor(img image.Image, level png.CompressionLevel) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: level}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodePNGExactPalette builds a paletted image from the image's unique colors
// (must be <= maxColors) and encodes it.
func encodePNGExactPalette(img image.Image, maxColors int, level png.CompressionLevel) ([]byte, error) {
	paletted, err := toExactPaletted(img, maxColors)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: level}
	if err := enc.Encode(&buf, paletted); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toExactPaletted(img image.Image, maxColors int) (*image.Paletted, error) {
	if maxColors < 1 {
		maxColors = 256
	}
	if maxColors > 256 {
		maxColors = 256
	}
	b := img.Bounds()
	index := make(map[uint32]uint8, maxColors)
	pal := make(color.Palette, 0, maxColors)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			c := color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(bl >> 8),
				A: uint8(a >> 8),
			}
			key := rgbaKey(c)
			if _, ok := index[key]; ok {
				continue
			}
			if len(pal) >= maxColors {
				return nil, fmt.Errorf("image has more than %d unique colors", maxColors)
			}
			index[key] = uint8(len(pal))
			pal = append(pal, c)
		}
	}

	out := image.NewPaletted(b, pal)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			key := rgbaKey(color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(bl >> 8),
				A: uint8(a >> 8),
			})
			out.SetColorIndex(x, y, index[key])
		}
	}
	return out, nil
}

func encodePNGLossyPalette(img image.Image, colors int, level png.CompressionLevel) ([]byte, error) {
	if colors < 1 {
		colors = 256
	}
	if colors > 256 {
		colors = 256
	}
	paletted := octreequant.Paletted(img, colors)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: level}
	if err := enc.Encode(&buf, paletted); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// crushPNG encodes PNG candidates per omakase rules and returns the smallest.
func crushPNG(img image.Image, cfg *config) ([]byte, string, error) {
	level := pngCompressionLevel(cfg.pngLevel)
	maxColors := cfg.pngColors
	if maxColors == 0 {
		maxColors = 256
	}

	kind := choosePNGKind(img, cfg.lossyPNG, cfg.noLossyPNG, maxColors)

	truecolor, err := encodePNGTruecolor(img, level)
	if err != nil {
		return nil, "", err
	}

	best := truecolor
	strategy := "truecolor"

	switch kind {
	case pngExactPalette:
		pal, err := encodePNGExactPalette(img, maxColors, level)
		if err != nil {
			return nil, "", err
		}
		if len(pal) < len(best) {
			best = pal
			strategy = "palette"
		}
	case pngLossyPalette:
		pal, err := encodePNGLossyPalette(img, maxColors, level)
		if err != nil {
			return nil, "", err
		}
		if len(pal) < len(best) {
			best = pal
			strategy = "palette-lossy"
		}
	}

	return best, strategy, nil
}
