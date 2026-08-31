package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func defaultCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "imgcrush")
}

func resolveCacheDir(cfg *config) string {
	if cfg.cacheDir != "" {
		return cfg.cacheDir
	}
	return defaultCacheDir()
}

// cacheAlgoVersion identifies the compression algorithm for the skip cache.
// Bump only when crush behavior changes; do not tie to CLI release version.
// Value "1.3.0" matches the fingerprint epoch when the skip cache shipped.
const cacheAlgoVersion = "1.3.0"

// settingsFingerprint returns an 8-hex digest of compression-relevant settings.
func settingsFingerprint(cfg *config) string {
	s := fmt.Sprintf("%s|q=%d|png=%d|colors=%d|lossy=%t|nolossy=%t|th=%.4f",
		cacheAlgoVersion, cfg.quality, cfg.pngLevel, cfg.pngColors,
		cfg.lossyPNG, cfg.noLossyPNG, cfg.threshold)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cachePath(dir, contentHash, fp string) string {
	if len(contentHash) < 2 {
		return filepath.Join(dir, contentHash+"-"+fp)
	}
	return filepath.Join(dir, contentHash[:2], contentHash+"-"+fp)
}

func cacheLookup(dir, contentHash, fp string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(cachePath(dir, contentHash, fp))
	return err == nil
}

// cacheStore records a marker file. Errors are ignored (best-effort).
func cacheStore(dir, contentHash, fp string) {
	if dir == "" {
		return
	}
	path := cachePath(dir, contentHash, fp)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	_ = os.WriteFile(path, nil, 0644)
}

func shouldReadCache(cfg *config) bool {
	return !cfg.force && !cfg.noCache
}

func shouldWriteCache(cfg *config) bool {
	return !cfg.noCache && !cfg.dryRun
}
