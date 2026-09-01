package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSplitCLIArgs(t *testing.T) {
	flags, files := splitCLIArgs([]string{"a.png", "--no-backup", "b.png", "--quiet"})
	if len(flags) != 2 || flags[0] != "--no-backup" || flags[1] != "--quiet" {
		t.Fatalf("flags = %v", flags)
	}
	if len(files) != 2 || files[0] != "a.png" || files[1] != "b.png" {
		t.Fatalf("files = %v", files)
	}

	flags, files = splitCLIArgs([]string{"--suffix", ".min", "photo.jpg"})
	if len(flags) != 2 || flags[0] != "--suffix" || flags[1] != ".min" {
		t.Fatalf("flags = %v", flags)
	}
	if len(files) != 1 || files[0] != "photo.jpg" {
		t.Fatalf("files = %v", files)
	}

	flags, files = splitCLIArgs([]string{"photo.jpg", "--outdir", "/tmp/out"})
	if len(flags) != 2 || flags[0] != "--outdir" || flags[1] != "/tmp/out" {
		t.Fatalf("flags = %v", flags)
	}
	if len(files) != 1 || files[0] != "photo.jpg" {
		t.Fatalf("files = %v", files)
	}

	flags, files = splitCLIArgs([]string{"--", "--no-backup", "x.png"})
	if len(flags) != 0 {
		t.Fatalf("flags after -- should be empty, got %v", flags)
	}
	if len(files) != 2 || files[0] != "--no-backup" || files[1] != "x.png" {
		t.Fatalf("files = %v", files)
	}
}

func TestCacheLookupStore(t *testing.T) {
	dir := t.TempDir()
	cfg := &config{quality: 85, pngLevel: 3, pngColors: 256, threshold: 10}
	fp := settingsFingerprint(cfg)
	data := []byte("hello-image-bytes")
	id := contentCacheID(data, fp)

	if cacheLookup(dir, cacheKindContent, id) {
		t.Fatal("expected miss")
	}
	cacheStore(dir, cacheKindContent, id)
	if !cacheLookup(dir, cacheKindContent, id) {
		t.Fatal("expected hit after store")
	}
	cfg2 := *cfg
	cfg2.quality = 70
	id2 := contentCacheID(data, settingsFingerprint(&cfg2))
	if cacheLookup(dir, cacheKindContent, id2) {
		t.Fatal("expected miss for different settings")
	}
}

func TestProcessFileCacheSkip(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	cfg := &config{
		quality:   85,
		pngLevel:  3,
		pngColors: 256,
		threshold: 10,
		noBackup:  true,
		cacheDir:  cacheDir,
	}

	r1 := processFile(src, cfg)
	if r1.err != nil {
		t.Fatalf("first pass: %v", r1.err)
	}
	if r1.skipped && r1.reason == "cached" {
		t.Fatal("first pass should not be cached")
	}

	// Second pass on (possibly rewritten) file should hit L0 cache.
	r2 := processFile(src, cfg)
	if r2.err != nil {
		t.Fatalf("second pass: %v", r2.err)
	}
	if !r2.skipped || r2.reason != "cached" {
		t.Fatalf("second pass: skipped=%v reason=%q, want cached", r2.skipped, r2.reason)
	}

	// --force bypasses cache read
	cfg.force = true
	r3 := processFile(src, cfg)
	if r3.err != nil {
		t.Fatalf("force pass: %v", r3.err)
	}
	if r3.reason == "cached" {
		t.Fatal("--force should not return cached")
	}

	// --no-cache neither reads nor writes
	cfg.force = false
	cfg.noCache = true
	fresh := filepath.Join(tmp, "fresh.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", fresh)
	emptyCache := t.TempDir()
	cfg.cacheDir = emptyCache
	_ = processFile(fresh, cfg)
	entries, _ := os.ReadDir(emptyCache)
	if len(entries) != 0 {
		t.Fatalf("--no-cache should not write cache, got %d entries", len(entries))
	}
}

func TestDryRunDoesNotWriteCache(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(tmp, "test.jpg")
	copyTestFile(t, "testdata/jpeg/large-cat-photo.jpg", src)

	cfg := &config{
		quality:   85,
		pngLevel:  3,
		pngColors: 256,
		threshold: 10,
		dryRun:    true,
		cacheDir:  cacheDir,
	}
	r := processFile(src, cfg)
	if r.err != nil {
		t.Fatal(r.err)
	}
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Fatal("dry-run must not write cache")
	}
}

func TestCacheL2HitAfterCopy(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(tmp, "a.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	cfg := &config{
		quality:   85,
		pngLevel:  3,
		pngColors: 256,
		threshold: 10,
		noBackup:  true,
		cacheDir:  cacheDir,
	}

	r1 := processFile(src, cfg)
	if r1.err != nil {
		t.Fatalf("first: %v", r1.err)
	}

	// Byte-identical copy at a new path (new inode) should hit L2 after read.
	dst := filepath.Join(tmp, "b.jpg")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatal(err)
	}

	r2 := processFile(dst, cfg)
	if r2.err != nil {
		t.Fatalf("copy pass: %v", r2.err)
	}
	if !r2.skipped || r2.reason != "cached" {
		t.Fatalf("copy: skipped=%v reason=%q, want cached via L2", r2.skipped, r2.reason)
	}
}

func TestCacheL0SurvivesRename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("inode L0 is path-based on Windows")
	}
	tmp := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(tmp, "orig.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	cfg := &config{
		quality:   85,
		pngLevel:  3,
		pngColors: 256,
		threshold: 10,
		noBackup:  true,
		cacheDir:  cacheDir,
	}

	r1 := processFile(src, cfg)
	if r1.err != nil {
		t.Fatalf("first: %v", r1.err)
	}

	renamed := filepath.Join(tmp, "renamed.jpg")
	if err := os.Rename(src, renamed); err != nil {
		t.Fatal(err)
	}

	r2 := processFile(renamed, cfg)
	if r2.err != nil {
		t.Fatalf("renamed: %v", r2.err)
	}
	if !r2.skipped || r2.reason != "cached" {
		t.Fatalf("rename: skipped=%v reason=%q, want cached via L0", r2.skipped, r2.reason)
	}
}

func TestCacheXattrHitBackfillsL0(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("xattr only on darwin/linux")
	}
	tmp := t.TempDir()
	cacheDir := t.TempDir()
	src := filepath.Join(tmp, "x.jpg")
	copyTestFile(t, "testdata/jpeg/small-sunrise.jpg", src)

	cfg := &config{
		quality:   85,
		pngLevel:  3,
		pngColors: 256,
		threshold: 10,
		noBackup:  true,
		cacheDir:  cacheDir,
	}
	fp := settingsFingerprint(cfg)

	r1 := processFile(src, cfg)
	if r1.err != nil {
		t.Fatalf("first: %v", r1.err)
	}
	got, ok := getSettledXattr(src)
	if !ok || got != fp {
		t.Fatalf("xattr after crush: ok=%v val=%q want %q", ok, got, fp)
	}

	// Clear central cache; xattr alone should settle and backfill L0.
	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	r2 := processFile(src, cfg)
	if r2.err != nil {
		t.Fatalf("xattr pass: %v", r2.err)
	}
	if !r2.skipped || r2.reason != "cached" {
		t.Fatalf("xattr: skipped=%v reason=%q, want cached", r2.skipped, r2.reason)
	}
}
