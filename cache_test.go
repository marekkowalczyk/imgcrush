package main

import (
	"os"
	"path/filepath"
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
	h := contentHash(data)

	if cacheLookup(dir, h, fp) {
		t.Fatal("expected miss")
	}
	cacheStore(dir, h, fp)
	if !cacheLookup(dir, h, fp) {
		t.Fatal("expected hit after store")
	}
	// Different settings fingerprint → miss
	cfg2 := *cfg
	cfg2.quality = 70
	if cacheLookup(dir, h, settingsFingerprint(&cfg2)) {
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

	// Second pass on (possibly rewritten) file should hit cache.
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
