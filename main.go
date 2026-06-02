package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type config struct {
	quality  int
	pngLevel int
	outdir   string
	suffix   string
	dryRun   bool
	force    bool
	noBackup bool
	quiet    bool
}

type result struct {
	file     string
	origSize int64
	newSize  int64
	skipped  bool
	reason   string
	err      error
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("imgcrush", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg config
	var showVersion bool

	fs.IntVar(&cfg.quality, "quality", 85, "JPEG quality (1-100)")
	fs.IntVar(&cfg.pngLevel, "png-level", 3, "PNG compression level (0=none, 1=fast, 2=default, 3=best)")
	fs.StringVar(&cfg.outdir, "outdir", "", "write output to this directory")
	fs.StringVar(&cfg.suffix, "suffix", "", "append suffix to output filenames (e.g. .min)")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "report what would happen without writing")
	fs.BoolVar(&cfg.force, "force", false, "compress even if gain is below 10%")
	fs.BoolVar(&cfg.noBackup, "no-backup", false, "skip creating .bak files in in-place mode")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress all output")
	fs.BoolVar(&showVersion, "version", false, "show version")
	fs.BoolVar(&showVersion, "v", false, "show version")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "imgcrush %s — compress JPEG and PNG images\n\n", version)
		fmt.Fprintf(stderr, "Usage: imgcrush [flags] <files...>\n\n")
		fmt.Fprintf(stderr, "Note: re-encoding strips all metadata (EXIF, ICC profiles, XMP).\n")
		fmt.Fprintf(stderr, "Back up originals if you need metadata preserved.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	if showVersion {
		fmt.Fprintf(stdout, "imgcrush %s\n", version)
		return 0
	}

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintf(stderr, "imgcrush: no files specified\n")
		fmt.Fprintf(stderr, "Run 'imgcrush --help' for usage.\n")
		return 1
	}

	if cfg.quality < 1 || cfg.quality > 100 {
		fmt.Fprintf(stderr, "imgcrush: --quality must be between 1 and 100\n")
		return 1
	}
	if cfg.pngLevel < 0 || cfg.pngLevel > 3 {
		fmt.Fprintf(stderr, "imgcrush: --png-level must be between 0 and 3\n")
		return 1
	}
	if cfg.outdir != "" && cfg.suffix != "" {
		fmt.Fprintf(stderr, "imgcrush: --outdir and --suffix are mutually exclusive\n")
		return 1
	}

	if cfg.outdir != "" {
		if err := os.MkdirAll(cfg.outdir, 0755); err != nil {
			fmt.Fprintf(stderr, "imgcrush: cannot create output directory: %v\n", err)
			return 1
		}
	}

	if !cfg.quiet {
		fmt.Fprintf(stderr, "imgcrush: metadata (EXIF, ICC, XMP) will be stripped\n")
	}

	var (
		totalOrig    int64
		totalNew     int64
		processed    int
		skipped      int
		errCount     int
	)

	for _, file := range files {
		r := processFile(file, &cfg)
		if r.err != nil {
			fmt.Fprintf(stderr, "imgcrush: %s: %v\n", r.file, r.err)
			errCount++
			continue
		}

		if r.skipped {
			if !cfg.quiet {
				fmt.Fprintf(stdout, "  skip  %s (%s)\n", r.file, r.reason)
			}
			skipped++
			continue
		}

		processed++
		totalOrig += r.origSize
		totalNew += r.newSize
		saved := float64(r.origSize-r.newSize) / float64(r.origSize) * 100

		if !cfg.quiet {
			fmt.Fprintf(stdout, "  %s  %s  %s -> %s (%.1f%%)\n",
				actionLabel(&cfg), r.file,
				humanSize(r.origSize), humanSize(r.newSize), saved)
		}
	}

	if !cfg.quiet && (processed > 0 || skipped > 0) {
		fmt.Fprintln(stdout)
		totalSaved := totalOrig - totalNew
		fmt.Fprintf(stdout, "  %d compressed, %d skipped, %d errors. Saved %s.\n",
			processed, skipped, errCount, humanSize(totalSaved))
	}

	if errCount > 0 {
		return 1
	}
	return 0
}

func actionLabel(cfg *config) string {
	if cfg.dryRun {
		return "dry "
	}
	return "ok  "
}

func processFile(path string, cfg *config) result {
	data, err := os.ReadFile(path)
	if err != nil {
		return result{file: path, err: fmt.Errorf("cannot read: %w", err)}
	}

	origSize := int64(len(data))

	format, err := detectFormat(data)
	if err != nil {
		return result{file: path, err: err}
	}

	img, err := decodeImage(data, format)
	if err != nil {
		return result{file: path, err: fmt.Errorf("cannot decode %s: %w", format, err)}
	}

	compressed, err := encodeImage(img, format, cfg)
	if err != nil {
		return result{file: path, err: fmt.Errorf("cannot encode %s: %w", format, err)}
	}

	newSize := int64(len(compressed))

	if newSize >= origSize {
		return result{file: path, origSize: origSize, skipped: true, reason: "already optimal"}
	}

	gain := float64(origSize-newSize) / float64(origSize) * 100
	if gain < 10 && !cfg.force {
		return result{file: path, origSize: origSize, skipped: true,
			reason: fmt.Sprintf("minimal gain: %.1f%%", gain)}
	}

	if cfg.dryRun {
		return result{file: path, origSize: origSize, newSize: newSize}
	}

	outPath := outputPath(path, cfg)

	if outPath == path && !cfg.noBackup {
		if err := copyFile(path, path+".bak"); err != nil {
			return result{file: path, err: fmt.Errorf("cannot create backup: %w", err)}
		}
	}

	if err := os.WriteFile(outPath, compressed, 0644); err != nil {
		return result{file: path, err: fmt.Errorf("cannot write: %w", err)}
	}

	return result{file: path, origSize: origSize, newSize: newSize}
}

func detectFormat(data []byte) (string, error) {
	r := bytes.NewReader(data)
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return "", fmt.Errorf("not a supported image format")
	}
	if format != "jpeg" && format != "png" {
		return "", fmt.Errorf("unsupported format: %s (only JPEG and PNG)", format)
	}
	return format, nil
}

func decodeImage(data []byte, format string) (image.Image, error) {
	r := bytes.NewReader(data)
	switch format {
	case "jpeg":
		return jpeg.Decode(r)
	case "png":
		return png.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func encodeImage(img image.Image, format string, cfg *config) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: cfg.quality}); err != nil {
			return nil, err
		}
	case "png":
		enc := png.Encoder{CompressionLevel: pngCompressionLevel(cfg.pngLevel)}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func pngCompressionLevel(level int) png.CompressionLevel {
	switch level {
	case 0:
		return png.NoCompression
	case 1:
		return png.BestSpeed
	case 2:
		return png.DefaultCompression
	case 3:
		return png.BestCompression
	default:
		return png.BestCompression
	}
}

func outputPath(path string, cfg *config) string {
	if cfg.outdir != "" {
		return filepath.Join(cfg.outdir, filepath.Base(path))
	}
	if cfg.suffix != "" {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)
		return base + cfg.suffix + ext
	}
	return path
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func humanSize(b int64) string {
	if b < 0 {
		return "0 B"
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
