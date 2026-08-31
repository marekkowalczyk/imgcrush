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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const version = "1.3.1"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type config struct {
	quality    int
	pngLevel   int
	outdir     string
	suffix     string
	dryRun     bool
	force      bool
	noBackup   bool
	noCache    bool
	quiet      bool
	lossyPNG   bool
	noLossyPNG bool
	pngColors  int
	threshold  float64
	cacheDir   string // empty → UserCacheDir()/imgcrush; tests inject a temp dir
}

type result struct {
	file     string
	origSize int64
	newSize  int64
	skipped  bool
	reason   string
	strategy string // PNG: truecolor, palette, palette-lossy
	err      error
}

// flagsWithValues names CLI flags that consume a following argument.
var flagsWithValues = map[string]bool{
	"quality":    true,
	"png-level":  true,
	"png-colors": true,
	"threshold":  true,
	"outdir":     true,
	"suffix":     true,
}

// splitCLIArgs separates flag tokens from file operands so flags may appear
// after filenames. Everything after a bare "--" is treated as a filename.
// Flags that take values keep their following argument in flagArgs.
func splitCLIArgs(args []string) (flagArgs, fileArgs []string) {
	seenDD := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if seenDD {
			fileArgs = append(fileArgs, a)
			continue
		}
		if a == "--" {
			seenDD = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			fileArgs = append(fileArgs, a)
			continue
		}

		flagArgs = append(flagArgs, a)
		name, inline := flagNameAndInline(a)
		if inline || !flagsWithValues[name] {
			continue
		}
		if i+1 < len(args) && args[i+1] != "--" {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, fileArgs
}

// flagNameAndInline returns the flag name and whether the value is inline (--x=y).
func flagNameAndInline(arg string) (name string, inline bool) {
	body := arg
	if strings.HasPrefix(arg, "--") {
		body = arg[2:]
	} else if strings.HasPrefix(arg, "-") {
		body = arg[1:]
	}
	if i := strings.IndexByte(body, '='); i >= 0 {
		return body[:i], true
	}
	return body, false
}

// parseArgs parses command-line flags and returns the config, file list, and
// an exit code. A non-negative exit code means run should return immediately.
func parseArgs(args []string, stdout, stderr io.Writer) (config, []string, int) {
	fs := flag.NewFlagSet("imgcrush", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg config
	var showVersion bool

	fs.IntVar(&cfg.quality, "quality", 85, "JPEG quality (1-100)")
	fs.IntVar(&cfg.pngLevel, "png-level", 3, "PNG compression level (0=none, 1=fast, 2=default, 3=best)")
	fs.IntVar(&cfg.pngColors, "png-colors", 256, "max palette size for lossy PNG (1-256)")
	fs.Float64Var(&cfg.threshold, "threshold", 10, "skip if size gain is below this percent")
	fs.StringVar(&cfg.outdir, "outdir", "", "write output to this directory")
	fs.StringVar(&cfg.suffix, "suffix", "", "append suffix to output filenames (e.g. .min)")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "report what would happen without writing")
	fs.BoolVar(&cfg.force, "force", false, "compress even if gain is below --threshold")
	fs.BoolVar(&cfg.lossyPNG, "lossy-png", false, "force palette quantization for PNG")
	fs.BoolVar(&cfg.noLossyPNG, "no-lossy-png", false, "disable automatic lossy PNG quantization")
	fs.BoolVar(&cfg.noBackup, "no-backup", false, "skip creating .bak files in in-place mode")
	fs.BoolVar(&cfg.noCache, "no-cache", false, "disable incremental skip cache")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress all output")
	fs.BoolVar(&showVersion, "version", false, "show version")
	fs.BoolVar(&showVersion, "v", false, "show version")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "imgcrush %s — compress JPEG and PNG images\n\n", version)
		fmt.Fprintf(stderr, "Usage: imgcrush [flags] <files...>\n\n")
		fmt.Fprintf(stderr, "Flags may appear before or after filenames. Use -- to pass\n")
		fmt.Fprintf(stderr, "filenames that start with -.\n\n")
		fmt.Fprintf(stderr, "Note: re-encoding strips all metadata (EXIF, ICC profiles, XMP).\n")
		fmt.Fprintf(stderr, "Back up originals if you need metadata preserved.\n")
		fmt.Fprintf(stderr, "PNG omakase may quantize logo-like images to a palette for smaller files.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	flagArgs, fileArgs := splitCLIArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return cfg, nil, 0
		}
		return cfg, nil, 1
	}
	if len(fs.Args()) > 0 {
		// flag.Parse should consume all flagArgs; leftover means a mistake.
		fmt.Fprintf(stderr, "imgcrush: unexpected arguments: %v\n", fs.Args())
		return cfg, nil, 1
	}

	if showVersion {
		fmt.Fprintf(stdout, "imgcrush %s\n", version)
		return cfg, nil, 0
	}

	files := fileArgs
	if len(files) == 0 {
		fmt.Fprintf(stderr, "imgcrush: no files specified\n")
		fmt.Fprintf(stderr, "Run 'imgcrush --help' for usage.\n")
		return cfg, nil, 1
	}

	if cfg.quality < 1 || cfg.quality > 100 {
		fmt.Fprintf(stderr, "imgcrush: --quality must be between 1 and 100\n")
		return cfg, nil, 1
	}
	if cfg.pngLevel < 0 || cfg.pngLevel > 3 {
		fmt.Fprintf(stderr, "imgcrush: --png-level must be between 0 and 3\n")
		return cfg, nil, 1
	}
	if cfg.outdir != "" && cfg.suffix != "" {
		fmt.Fprintf(stderr, "imgcrush: --outdir and --suffix are mutually exclusive\n")
		return cfg, nil, 1
	}
	if cfg.lossyPNG && cfg.noLossyPNG {
		fmt.Fprintf(stderr, "imgcrush: --lossy-png and --no-lossy-png are mutually exclusive\n")
		return cfg, nil, 1
	}
	if cfg.pngColors < 1 || cfg.pngColors > 256 {
		fmt.Fprintf(stderr, "imgcrush: --png-colors must be between 1 and 256\n")
		return cfg, nil, 1
	}
	if cfg.threshold < 0 || cfg.threshold > 100 {
		fmt.Fprintf(stderr, "imgcrush: --threshold must be between 0 and 100\n")
		return cfg, nil, 1
	}

	return cfg, files, -1
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, files, exitCode := parseArgs(args, stdout, stderr)
	if exitCode >= 0 {
		return exitCode
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

	tty := writerIsTTY(stderr)
	var (
		printMu   sync.Mutex
		totalOrig int64
		totalNew  int64
		processed int
		skipped   int
		errCount  int
	)
	totalFiles := len(files)

	results := processFiles(files, &cfg, func(r result, done int) {
		printMu.Lock()
		defer printMu.Unlock()

		// Clear the in-place progress line before printing a result.
		if !cfg.quiet && tty && totalFiles >= 2 {
			fmt.Fprint(stderr, "\r\033[K")
		}

		if r.err != nil {
			fmt.Fprintf(stderr, "imgcrush: %s: %v\n", r.file, r.err)
			errCount++
		} else if r.skipped {
			if !cfg.quiet {
				fmt.Fprintf(stdout, "  skip  %s (%s)\n", r.file, r.reason)
			}
			skipped++
		} else {
			processed++
			totalOrig += r.origSize
			totalNew += r.newSize
			saved := float64(r.origSize-r.newSize) / float64(r.origSize) * 100
			if !cfg.quiet {
				if r.strategy != "" && r.strategy != "truecolor" {
					fmt.Fprintf(stdout, "  %s  %s  %s -> %s (%.1f%%, %s)\n",
						actionLabel(&cfg), r.file,
						humanSize(r.origSize), humanSize(r.newSize), saved, r.strategy)
				} else {
					fmt.Fprintf(stdout, "  %s  %s  %s -> %s (%.1f%%)\n",
						actionLabel(&cfg), r.file,
						humanSize(r.origSize), humanSize(r.newSize), saved)
				}
			}
		}

		// Single updating progress line on a TTY (no 1/N…N/N scroll litany).
		if !cfg.quiet && tty && totalFiles >= 2 {
			fmt.Fprintf(stderr, "\rimgcrush: %d/%d", done, totalFiles)
		}
	})
	_ = results

	if !cfg.quiet && tty && totalFiles >= 2 {
		fmt.Fprintln(stderr)
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

// processFiles compresses files concurrently, one worker per CPU, and
// returns results in the same order as the input files.
// onDone is called once per file as it finishes (completion order) with the
// 1-based count of completed files; it may be nil.
func processFiles(files []string, cfg *config, onDone func(result, int)) []result {
	collisions := outdirCollisions(files, cfg)

	results := make([]result, len(files))
	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}

	var done atomic.Int64
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				file := files[i]
				if collisions[file] {
					results[i] = result{file: file, err: fmt.Errorf(
						"output path collision in --outdir (duplicate basename %q among inputs)",
						filepath.Base(file))}
				} else {
					results[i] = processFile(file, cfg)
				}
				if onDone != nil {
					n := int(done.Add(1))
					onDone(results[i], n)
				}
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// outdirCollisions reports which input files would clobber each other's
// output when written to a shared --outdir (same base filename).
func outdirCollisions(files []string, cfg *config) map[string]bool {
	collisions := make(map[string]bool)
	if cfg.outdir == "" {
		return collisions
	}
	counts := make(map[string]int)
	for _, f := range files {
		counts[filepath.Base(f)]++
	}
	for _, f := range files {
		if counts[filepath.Base(f)] > 1 {
			collisions[f] = true
		}
	}
	return collisions
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
	cdir := resolveCacheDir(cfg)
	fp := settingsFingerprint(cfg)
	hash := contentHash(data)

	if shouldReadCache(cfg) && cacheLookup(cdir, hash, fp) {
		return result{file: path, origSize: origSize, skipped: true, reason: "cached"}
	}

	format, err := detectFormat(data)
	if err != nil {
		return result{file: path, err: err}
	}

	img, err := decodeImage(data, format)
	if err != nil {
		return result{file: path, err: fmt.Errorf("cannot decode %s: %w", format, err)}
	}

	compressed, strategy, err := compressImage(img, format, cfg)
	if err != nil {
		return result{file: path, err: fmt.Errorf("cannot encode %s: %w", format, err)}
	}

	newSize := int64(len(compressed))

	if newSize >= origSize {
		if shouldWriteCache(cfg) {
			cacheStore(cdir, hash, fp)
		}
		return result{file: path, origSize: origSize, skipped: true, reason: "already optimal"}
	}

	gain := float64(origSize-newSize) / float64(origSize) * 100
	if gain < cfg.threshold && !cfg.force {
		if shouldWriteCache(cfg) {
			cacheStore(cdir, hash, fp)
		}
		return result{file: path, origSize: origSize, skipped: true,
			reason: fmt.Sprintf("minimal gain: %.1f%%", gain)}
	}

	if cfg.dryRun {
		return result{file: path, origSize: origSize, newSize: newSize, strategy: strategy}
	}

	outPath := outputPath(path, cfg)

	if outPath == path && !cfg.noBackup {
		if err := os.WriteFile(path+".bak", data, 0644); err != nil {
			return result{file: path, err: fmt.Errorf("cannot create backup: %w", err)}
		}
	}

	if err := os.WriteFile(outPath, compressed, 0644); err != nil {
		return result{file: path, err: fmt.Errorf("cannot write: %w", err)}
	}

	if shouldWriteCache(cfg) {
		cacheStore(cdir, contentHash(compressed), fp)
	}

	return result{file: path, origSize: origSize, newSize: newSize, strategy: strategy}
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

// compressImage encodes img for the given format. For PNG it runs the omakase
// tournament and returns the winning strategy name.
func compressImage(img image.Image, format string, cfg *config) ([]byte, string, error) {
	switch format {
	case "jpeg":
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: cfg.quality}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "", nil
	case "png":
		return crushPNG(img, cfg)
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", format)
	}
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
