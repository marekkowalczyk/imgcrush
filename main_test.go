package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-v"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if got := stdout.String(); got != "imgcrush "+version+"\n" {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestNonImageFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"testdata/edge/not-an-image.txt"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	src := "testdata/jpeg/large-cat-photo.jpg"
	before, _ := os.Stat(src)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--dry-run", src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	after, _ := os.Stat(src)
	if before.Size() != after.Size() {
		t.Fatal("dry-run modified the file")
	}

	bakPath := src + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		os.Remove(bakPath)
		t.Fatal("dry-run created a backup file")
	}
}

func TestSuffixMode(t *testing.T) {
	src := "testdata/jpeg/large-cat-photo.jpg"
	expected := "testdata/jpeg/large-cat-photo.min.jpg"
	defer os.Remove(expected)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--suffix", ".min", src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	info, err := os.Stat(expected)
	if err != nil {
		t.Fatalf("suffix output not created: %v", err)
	}
	origInfo, _ := os.Stat(src)
	if info.Size() >= origInfo.Size() {
		t.Fatalf("suffix output (%d) not smaller than original (%d)", info.Size(), origInfo.Size())
	}
}

func TestOutdirMode(t *testing.T) {
	outdir := t.TempDir()
	src := "testdata/jpeg/large-cat-photo.jpg"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--outdir", outdir, src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	outFile := filepath.Join(outdir, "large-cat-photo.jpg")
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("outdir output not created: %v", err)
	}
	origInfo, _ := os.Stat(src)
	if info.Size() >= origInfo.Size() {
		t.Fatalf("outdir output (%d) not smaller than original (%d)", info.Size(), origInfo.Size())
	}
}

func TestInPlaceCreatesBackup(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	origData, _ := os.ReadFile(src)

	var stdout, stderr bytes.Buffer
	code := run([]string{src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	bakData, err := os.ReadFile(src + ".bak")
	if err != nil {
		t.Fatal("backup not created")
	}
	if !bytes.Equal(bakData, origData) {
		t.Fatal("backup content differs from original")
	}

	newData, _ := os.ReadFile(src)
	if len(newData) >= len(origData) {
		t.Fatal("in-place file not smaller after compression")
	}
}

func TestInPlaceNoBackup(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--no-backup", src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	if _, err := os.Stat(src + ".bak"); err == nil {
		t.Fatal("backup created despite --no-backup")
	}
}

func TestNoBackupAfterFilename(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	var stdout, stderr bytes.Buffer
	code := run([]string{src, "--no-backup"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(src + ".bak"); err == nil {
		t.Fatal("backup created despite trailing --no-backup")
	}
	if bytes.Contains(stderr.Bytes(), []byte("cannot read")) {
		t.Fatalf("treated --no-backup as a file: %s", stderr.String())
	}
}

func TestTrailingQuietFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--dry-run", "testdata/jpeg/large-cat-photo.jpg", "--quiet"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("quiet with trailing flag produced output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDoubleDashTreatsFlagAsFilename(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--", "--no-backup"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("cannot read")) &&
		!bytes.Contains(stderr.Bytes(), []byte("--no-backup")) {
		t.Fatalf("expected --no-backup treated as filename, got: %s", stderr.String())
	}
}

func TestSkipAlreadyOptimal(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	// First pass: compress
	var stdout1, stderr1 bytes.Buffer
	run([]string{"--no-backup", src}, &stdout1, &stderr1)

	// Second pass: should skip
	var stdout2, stderr2 bytes.Buffer
	code := run([]string{"--no-backup", src}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !bytes.Contains(stdout2.Bytes(), []byte("skip")) {
		t.Fatalf("expected skip on second run, got: %s", stdout2.String())
	}
}

func TestNeverGrows(t *testing.T) {
	// Even with --force, files that would grow must be skipped
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.png")
	copyTestFile(t, "testdata/png/transparency-demo.png", src)

	origInfo, _ := os.Stat(src)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--force", "--no-backup", src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	afterInfo, _ := os.Stat(src)
	if afterInfo.Size() > origInfo.Size() {
		t.Fatal("file grew after compression")
	}
}

func TestQuietMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--quiet", "--dry-run", "testdata/jpeg/large-cat-photo.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("quiet mode produced stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("quiet mode produced stderr: %q", stderr.String())
	}
}

func TestStreamingSkipsAppearLive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--dry-run",
		"testdata/jpeg/large-cat-photo.jpg",
		"testdata/jpeg/small-sunrise.jpg",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	// bytes.Buffer is not a TTY — no N/total litany; per-file lines are the signal.
	if bytes.Contains(stderr.Bytes(), []byte("imgcrush: 1/")) {
		t.Fatalf("non-TTY should not print N/total progress: %q", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("large-cat-photo.jpg")) ||
		!bytes.Contains(stdout.Bytes(), []byte("small-sunrise.jpg")) {
		t.Fatalf("expected per-file output, got: %q", stdout.String())
	}
}

func TestWriterIsTTY(t *testing.T) {
	var buf bytes.Buffer
	if writerIsTTY(&buf) {
		t.Fatal("bytes.Buffer should not count as TTY")
	}
}

func TestPNGCompression(t *testing.T) {
	outdir := t.TempDir()
	src := "testdata/png/large-gradient.png"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--outdir", outdir, src}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	outFile := filepath.Join(outdir, "large-gradient.png")
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("output not created: %v", err)
	}
	origInfo, _ := os.Stat(src)
	if info.Size() >= origInfo.Size() {
		t.Fatalf("PNG output (%d) not smaller than original (%d)", info.Size(), origInfo.Size())
	}
}

func TestOutdirCollision(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	outdir := t.TempDir()

	srcA := filepath.Join(dirA, "img.jpg")
	srcB := filepath.Join(dirB, "img.jpg")
	copyTestFile(t, "testdata/jpeg/large-cat-photo.jpg", srcA)
	copyTestFile(t, "testdata/jpeg/large-cat-photo.jpg", srcB)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--outdir", outdir, srcA, srcB}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for outdir collision, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("collision")) {
		t.Fatalf("expected collision error in stderr, got: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outdir, "img.jpg")); err == nil {
		t.Fatal("colliding inputs should not have written an output file")
	}
}

func TestMutuallyExclusiveFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--outdir", "/tmp", "--suffix", ".min", "test.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatal("expected exit 1 for mutually exclusive flags")
	}
}

func TestInvalidQuality(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--quality", "200", "test.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatal("expected exit 1 for invalid quality")
	}
}

func TestOmakaseFewColorsUsesPalette(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--dry-run", "testdata/png/simple-few-colors.png"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("palette")) {
		t.Fatalf("expected palette strategy in output, got: %s", stdout.String())
	}
}

func TestLossyPNGFlagsMutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--lossy-png", "--no-lossy-png", "testdata/png/simple-few-colors.png"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("mutually exclusive")) {
		t.Fatalf("expected mutual exclusion error, got: %s", stderr.String())
	}
}

func TestThresholdSkipsLowGain(t *testing.T) {
	// After first compression of a JPEG, second run with high threshold should skip.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	var stdout1, stderr1 bytes.Buffer
	if code := run([]string{"--no-backup", src}, &stdout1, &stderr1); code != 0 {
		t.Fatalf("first pass: %d %s", code, stderr1.String())
	}

	var stdout2, stderr2 bytes.Buffer
	code := run([]string{"--no-backup", "--threshold", "50", src}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr2.String())
	}
	if !bytes.Contains(stdout2.Bytes(), []byte("skip")) {
		t.Fatalf("expected skip with high threshold, got: %s", stdout2.String())
	}
}

func TestNoLossyPNGStillAllowsExactPalette(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--dry-run", "--no-lossy-png", "testdata/png/simple-few-colors.png"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("palette")) {
		t.Fatalf("exact palette should still run with --no-lossy-png, got: %s", stdout.String())
	}
}

func TestInvalidPNGColors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--png-colors", "512", "testdata/png/simple-few-colors.png"}, &stdout, &stderr)
	if code != 1 {
		t.Fatal("expected exit 1 for invalid --png-colors")
	}
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("cannot read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("cannot write %s: %v", dst, err)
	}
}
