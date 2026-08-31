package main

import (
	"image"
	"image/color"
)

const pngLossyUniqueCap = 2048

type pngKind int

const (
	pngTruecolor pngKind = iota
	pngExactPalette
	pngLossyPalette
)

func (k pngKind) String() string {
	switch k {
	case pngExactPalette:
		return "palette"
	case pngLossyPalette:
		return "palette-lossy"
	default:
		return "truecolor"
	}
}

// rgbaKey packs an NRGBA color into a uint32 for set membership.
func rgbaKey(c color.NRGBA) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

// countUniqueRGBA counts distinct RGBA colors, stopping once it reaches limit.
func countUniqueRGBA(img image.Image, limit int) int {
	if limit <= 0 {
		return 0
	}
	seen := make(map[uint32]struct{}, min(limit, 256))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			key := rgbaKey(color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(bl >> 8),
				A: uint8(a >> 8),
			})
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if len(seen) >= limit {
				return len(seen)
			}
		}
	}
	return len(seen)
}

// hasPartialAlpha reports whether any pixel has alpha not in {0, 255}.
func hasPartialAlpha(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			a8 := uint8(a >> 8)
			if a8 != 0 && a8 != 255 {
				return true
			}
		}
	}
	return false
}

// choosePNGKind selects the omakase PNG strategy for img.
// maxColors is the exact-palette / lossy target size (typically 256).
func choosePNGKind(img image.Image, forceLossy, noLossy bool, maxColors int) pngKind {
	if maxColors < 1 {
		maxColors = 256
	}
	if maxColors > 256 {
		maxColors = 256
	}

	uniques := countUniqueRGBA(img, pngLossyUniqueCap+1)

	if uniques <= maxColors {
		return pngExactPalette
	}
	if noLossy && !forceLossy {
		return pngTruecolor
	}
	if forceLossy {
		return pngLossyPalette
	}
	if uniques <= pngLossyUniqueCap && !hasPartialAlpha(img) {
		return pngLossyPalette
	}
	return pngTruecolor
}
