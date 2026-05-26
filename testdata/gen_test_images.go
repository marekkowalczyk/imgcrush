//go:build ignore

// gen_test_images.go generates test images for imgcrush testing.
// Run: go run testdata/gen_test_images.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"os"
)

func main() {
	generate("testdata/png/large-gradient.png", largePNGGradient)
	generate("testdata/png/simple-few-colors.png", simpleFewColors)
	generate("testdata/png/transparent-logo.png", transparentLogo)
	generate("testdata/edge/tiny-1x1.png", tiny1x1PNG)
	generate("testdata/edge/tiny-1x1.jpg", tiny1x1JPEG)
	generate("testdata/edge/noisy.jpg", noisyJPEG)
	fmt.Println("done")
}

func generate(path string, fn func(string) error) {
	if err := fn(path); err != nil {
		fmt.Fprintf(os.Stderr, "error generating %s: %v\n", path, err)
		os.Exit(1)
	}
	fi, _ := os.Stat(path)
	fmt.Printf("  %-40s %s\n", path, humanSize(fi.Size()))
}

func humanSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}

// Large PNG with smooth gradients — compresses well but is big.
func largePNGGradient(path string) error {
	w, h := 2000, 1500
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8(float64(x) / float64(w) * 255)
			g := uint8(float64(y) / float64(h) * 255)
			b := uint8(128 + 127*math.Sin(float64(x+y)/100.0))
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return writePNG(path, img, png.DefaultCompression)
}

// Simple PNG with very few colors — ideal compression candidate.
func simpleFewColors(path string) error {
	w, h := 400, 300
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	colors := []color.RGBA{
		{255, 255, 255, 255},
		{0, 120, 200, 255},
		{240, 80, 40, 255},
		{40, 180, 80, 255},
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (x / 100) % len(colors)
			img.SetRGBA(x, y, colors[idx])
		}
	}
	return writePNG(path, img, png.DefaultCompression)
}

// PNG with transparency — logo-style image with alpha channel.
func transparentLogo(path string) error {
	w, h := 500, 500
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 200 {
				alpha := uint8(255 - dist/200*128)
				img.SetRGBA(x, y, color.RGBA{50, 100, 200, alpha})
			}
			// else: transparent (zero value)
		}
	}
	return writePNG(path, img, png.DefaultCompression)
}

// 1x1 pixel PNG — edge case.
func tiny1x1PNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{128, 128, 128, 255})
	return writePNG(path, img, png.DefaultCompression)
}

// 1x1 pixel JPEG — edge case.
func tiny1x1JPEG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{128, 128, 128, 255})
	return writeJPEG(path, img, 85)
}

// Noisy JPEG — random pixels, very hard to compress.
func noisyJPEG(path string) error {
	w, h := 800, 600
	r := rand.New(rand.NewSource(42))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				uint8(r.Intn(256)),
				uint8(r.Intn(256)),
				uint8(r.Intn(256)),
				255,
			})
		}
	}
	return writeJPEG(path, img, 95)
}

func writePNG(path string, img image.Image, level png.CompressionLevel) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: level}
	return enc.Encode(f, img)
}

func writeJPEG(path string, img image.Image, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}
