//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// inodeCacheID on Windows falls back to path + size + mtime (no reliable inode).
func inodeCacheID(absPath string, fi os.FileInfo, fp string) string {
	s := fmt.Sprintf("%s|%d|%d|%s", absPath, fi.Size(), fi.ModTime().UnixNano(), fp)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
