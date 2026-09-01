//go:build !darwin && !linux

package main

func getSettledXattr(path string) (string, bool) {
	return "", false
}

func setSettledXattr(path, fp string) {}
