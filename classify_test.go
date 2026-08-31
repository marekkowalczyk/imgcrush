package main

import (
	"image"
	"image/color"
	"testing"
)

func solidImage(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func fewColorImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 30))
	colors := []color.NRGBA{
		{255, 255, 255, 255},
		{0, 120, 200, 255},
		{240, 80, 40, 255},
		{40, 180, 80, 255},
	}
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.SetNRGBA(x, y, colors[(x/10)%len(colors)])
		}
	}
	return img
}

func opaqueColorsImage(n int) *image.NRGBA {
	// n distinct opaque colors in a strip
	img := image.NewNRGBA(image.Rect(0, 0, n, 1))
	for i := 0; i < n; i++ {
		img.SetNRGBA(i, 0, color.NRGBA{
			R: uint8(i % 256),
			G: uint8((i / 256) % 256),
			B: uint8((i * 3) % 256),
			A: 255,
		})
	}
	return img
}

func partialAlphaManyColors() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 4),
				G: uint8(y * 4),
				B: 128,
				A: uint8(1 + (x+y)%254), // never 0 or 255
			})
		}
	}
	return img
}

func TestCountUniqueRGBA(t *testing.T) {
	img := fewColorImage()
	got := countUniqueRGBA(img, 257)
	if got != 4 {
		t.Fatalf("countUniqueRGBA = %d, want 4", got)
	}
}

func TestCountUniqueRGBAEarlyExit(t *testing.T) {
	img := opaqueColorsImage(300)
	got := countUniqueRGBA(img, 257)
	if got != 257 {
		t.Fatalf("countUniqueRGBA early exit = %d, want 257", got)
	}
}

func TestHasPartialAlpha(t *testing.T) {
	if hasPartialAlpha(fewColorImage()) {
		t.Fatal("few-color opaque image should not have partial alpha")
	}
	if !hasPartialAlpha(partialAlphaManyColors()) {
		t.Fatal("expected partial alpha")
	}
	binary := solidImage(2, 2, color.NRGBA{0, 0, 0, 0})
	binary.SetNRGBA(1, 1, color.NRGBA{255, 0, 0, 255})
	if hasPartialAlpha(binary) {
		t.Fatal("binary alpha only should not count as partial")
	}
}

func TestChoosePNGKindExact(t *testing.T) {
	kind := choosePNGKind(fewColorImage(), false, false, 256)
	if kind != pngExactPalette {
		t.Fatalf("got %v, want pngExactPalette", kind)
	}
}

func TestChoosePNGKindTruecolorManyColors(t *testing.T) {
	img := opaqueColorsImage(3000)
	kind := choosePNGKind(img, false, false, 256)
	if kind != pngTruecolor {
		t.Fatalf("got %v, want pngTruecolor", kind)
	}
}

func TestChoosePNGKindLossyMidBand(t *testing.T) {
	img := opaqueColorsImage(300)
	kind := choosePNGKind(img, false, false, 256)
	if kind != pngLossyPalette {
		t.Fatalf("got %v, want pngLossyPalette", kind)
	}
}

func TestChoosePNGKindPartialAlphaBlocksAutoLossy(t *testing.T) {
	img := partialAlphaManyColors()
	kind := choosePNGKind(img, false, false, 256)
	if kind != pngTruecolor {
		t.Fatalf("got %v, want pngTruecolor (partial alpha)", kind)
	}
	forced := choosePNGKind(img, true, false, 256)
	if forced != pngLossyPalette {
		t.Fatalf("forceLossy got %v, want pngLossyPalette", forced)
	}
}

func TestChoosePNGKindNoLossyStillAllowsExact(t *testing.T) {
	exact := choosePNGKind(fewColorImage(), false, true, 256)
	if exact != pngExactPalette {
		t.Fatalf("noLossy exact got %v, want pngExactPalette", exact)
	}
	mid := choosePNGKind(opaqueColorsImage(300), false, true, 256)
	if mid != pngTruecolor {
		t.Fatalf("noLossy mid-band got %v, want pngTruecolor", mid)
	}
}
