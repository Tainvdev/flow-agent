// Package atomicfile publishes complete file contents with an atomic rename.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Windows can transiently reject two simultaneous replace operations for the
// same destination. Staging remains concurrent; only the short publication
// syscall is serialized.
var publishMu sync.Mutex

// Write creates a uniquely named temporary file beside path, flushes its
// contents, then atomically replaces path. Readers see either the previous
// complete file or the new complete file, never a partially written result.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomicfile: create directory: %w", err)
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("atomicfile: create temporary file: %w", err)
	}
	tmp := f.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return fmt.Errorf("atomicfile: set permissions: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("atomicfile: write temporary file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("atomicfile: sync temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("atomicfile: close temporary file: %w", err)
	}
	publishMu.Lock()
	err = replace(tmp, path)
	publishMu.Unlock()
	if err != nil {
		return fmt.Errorf("atomicfile: publish file: %w", err)
	}
	committed = true
	return nil
}
