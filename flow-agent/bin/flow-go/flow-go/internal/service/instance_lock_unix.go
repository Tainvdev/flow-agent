//go:build !windows

package service

import (
	"os"
	"syscall"
)

func lockFile(file *os.File) (func(*os.File) error, error) {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	return func(file *os.File) error { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, nil
}
