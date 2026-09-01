//go:build linux

package main

import "golang.org/x/sys/unix"

const settledXattrName = "user.imgcrush"

func getSettledXattr(path string) (string, bool) {
	buf := make([]byte, 64)
	n, err := unix.Getxattr(path, settledXattrName, buf)
	if err == unix.ERANGE {
		n, err = unix.Getxattr(path, settledXattrName, nil)
		if err != nil || n <= 0 {
			return "", false
		}
		buf = make([]byte, n)
		n, err = unix.Getxattr(path, settledXattrName, buf)
	}
	if err != nil || n <= 0 {
		return "", false
	}
	return string(buf[:n]), true
}

func setSettledXattr(path, fp string) {
	_ = unix.Setxattr(path, settledXattrName, []byte(fp), 0)
}
