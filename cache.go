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

// cacheAlgoVersion identifies the skip-cache keying algorithm.
// Bump only when crush behavior or cache key semantics change; do not tie to CLI semver.
// 1.3.0 = content-hash of file bytes
// 1.4.0 = path + size + mtime (Stat-only hits)
// 1.5.0 = cascade: L0 inode Stat, L0b xattr, L2 content hash
const cacheAlgoVersion = "1.5.0"

const (
	cacheKindInode   = "i"
	cacheKindContent = "c"
)

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

// contentCacheID is the L2 key: content hash + settings fingerprint.
func contentCacheID(data []byte, fp string) string {
	s := contentHash(data) + "|" + fp
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func cacheMarkerPath(dir, kind, id string) string {
	if len(id) < 2 {
		return filepath.Join(dir, kind, id)
	}
	return filepath.Join(dir, kind, id[:2], id)
}

func cacheLookup(dir, kind, id string) bool {
	if dir == "" || id == "" {
		return false
	}
	_, err := os.Stat(cacheMarkerPath(dir, kind, id))
	return err == nil
}

// cacheStore records a marker file. Errors are ignored (best-effort).
func cacheStore(dir, kind, id string) {
	if dir == "" || id == "" {
		return
	}
	path := cacheMarkerPath(dir, kind, id)
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

// storeInodeCache writes L0 for path's current Stat identity and best-effort xattr.
func storeInodeCache(dir, path, fp string) {
	if dir == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	cacheStore(dir, cacheKindInode, inodeCacheID(abs, fi, fp))
	setSettledXattr(path, fp)
}

// storeContentCache writes L2 for the given bytes.
func storeContentCache(dir string, data []byte, fp string) {
	if dir == "" {
		return
	}
	cacheStore(dir, cacheKindContent, contentCacheID(data, fp))
}

// storeSettledCache records L0 + L2 + xattr for a settled file at path with data.
func storeSettledCache(dir, path, fp string, data []byte) {
	storeContentCache(dir, data, fp)
	storeInodeCache(dir, path, fp)
}
