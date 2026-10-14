//go:build windows

package service

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(file *os.File) (func(*os.File) error, error) {
	var overlapped windows.Overlapped
	const exclusive = 0x00000002
	const failImmediately = 0x00000001
	if err := windows.LockFileEx(windows.Handle(file.Fd()), exclusive|failImmediately, 0, 1, 0, &overlapped); err != nil {
		return nil, err
	}
	return func(file *os.File) error {
		return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	}, nil
}
