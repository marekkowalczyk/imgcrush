package main

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

func TestEncodePNGExactPaletteSmallerThanTruecolor(t *testing.T) {
	f, err := os.Open("testdata/png/simple-few-colors.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	truecolor, err := encodePNGTruecolor(img, png.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := encodePNGExactPalette(img, 256, png.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if len(palette) >= len(truecolor) {
		t.Fatalf("exact palette (%d) should be smaller than truecolor (%d)", len(palette), len(truecolor))
	}
}

func TestCrushPNGFewColorsPicksPalette(t *testing.T) {
	f, err := os.Open("testdata/png/simple-few-colors.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config{pngLevel: 3, pngColors: 256}
	data, strategy, err := crushPNG(img, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strategy != "palette" {
		t.Fatalf("strategy = %q, want palette", strategy)
	}
	truecolor, _ := encodePNGTruecolor(img, png.BestCompression)
	if len(data) > len(truecolor) {
		t.Fatal("crushPNG returned larger than truecolor candidate")
	}
}

func TestCrushPNGGradientStaysTruecolor(t *testing.T) {
	f, err := os.Open("testdata/png/large-gradient.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	kind := choosePNGKind(img, false, false, 256)
	if kind != pngTruecolor {
		t.Fatalf("classifier kind = %v, want pngTruecolor", kind)
	}

	cfg := &config{pngLevel: 3, pngColors: 256}
	_, strategy, err := crushPNG(img, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strategy != "truecolor" {
		t.Fatalf("strategy = %q, want truecolor", strategy)
	}
}

func TestCrushPNGLossyMidBand(t *testing.T) {
	img := opaqueColorsImage(300)
	cfg := &config{pngLevel: 3, pngColors: 256}
	data, strategy, err := crushPNG(img, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strategy != "palette-lossy" && strategy != "truecolor" {
		t.Fatalf("unexpected strategy %q", strategy)
	}
	// Winner must be the smallest candidate we would produce.
	truecolor, err := encodePNGTruecolor(img, png.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	lossy, err := encodePNGLossyPalette(img, 256, png.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	wantLen := len(truecolor)
	wantStrat := "truecolor"
	if len(lossy) < wantLen {
		wantLen = len(lossy)
		wantStrat = "palette-lossy"
	}
	if len(data) != wantLen || strategy != wantStrat {
		t.Fatalf("got %s/%d, want %s/%d", strategy, len(data), wantStrat, wantLen)
	}
}

func TestEncodePNGExactPaletteRoundTripColors(t *testing.T) {
	img := fewColorImage()
	data, err := encodePNGExactPalette(img, 256, png.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	out, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			ir, ig, ib, ia := img.At(x, y).RGBA()
			or, og, ob, oa := out.At(x, y).RGBA()
			if ir != or || ig != og || ib != ob || ia != oa {
				t.Fatalf("pixel mismatch at %d,%d", x, y)
			}
		}
	}
}
