package service

import (
	"fmt"
	"os"
)

// InstanceLock owns a process-wide lock file for one service data directory.
// The OS releases the lock automatically if the process exits unexpectedly.
type InstanceLock struct {
	file    *os.File
	release func(*os.File) error
}

func AcquireInstanceLock(path string) (*InstanceLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("service: open instance lock: %w", err)
	}
	release, err := lockFile(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("service: another process owns the service lock %q: %w", path, err)
	}
	if err := file.Truncate(0); err == nil {
		_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	}
	return &InstanceLock{file: file, release: release}, nil
}

func (l *InstanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := l.release(l.file)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
