package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "media.mp4")
	if err := os.WriteFile(path, []byte("old complete contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []byte("new complete contents")
	if err := Write(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("published contents = %q, want %q", got, want)
	}
}

func TestConcurrentWritersPublishOnlyCompleteFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "media.mp4")
	values := [][]byte{bytes.Repeat([]byte("A"), 64*1024), bytes.Repeat([]byte("B"), 64*1024)}
	var wg sync.WaitGroup
	errCh := make(chan error, len(values)*8)
	for i := 0; i < 8; i++ {
		for _, value := range values {
			wg.Add(1)
			go func(data []byte) {
				defer wg.Done()
				errCh <- Write(path, data, 0o644)
			}(value)
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, values[0]) && !bytes.Equal(got, values[1]) {
		t.Fatalf("published file is partial or mixed: length=%d", len(got))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "media.mp4" {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}
