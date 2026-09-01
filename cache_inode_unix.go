//go:build unix

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"syscall"
)

// inodeCacheID is the L0 Stat-only key: device + inode + size + mtime + settings.
// Survives rename on the same volume; works on iCloud placeholders without download.
func inodeCacheID(absPath string, fi os.FileInfo, fp string) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return pathFallbackCacheID(absPath, fi, fp)
	}
	s := fmt.Sprintf("%d|%d|%d|%d|%s", st.Dev, st.Ino, fi.Size(), fi.ModTime().UnixNano(), fp)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func pathFallbackCacheID(absPath string, fi os.FileInfo, fp string) string {
	s := fmt.Sprintf("%s|%d|%d|%s", absPath, fi.Size(), fi.ModTime().UnixNano(), fp)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
