package submission

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeDurable writes data to path, calls Sync to flush the file's bytes to stable
// storage (so it survives a power loss), then closes. It opens with O_CREATE|O_TRUNC
// (overwriting any existing file) and creates parent directories as needed. Used before
// an atomic rename so the renamed file is guaranteed durable on disk.
func writeDurable(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("writeDurable: mkdir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("writeDurable: create %s: %w", path, err)
	}
	// Close on every return path; report write/sync/close errors in priority.
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("writeDurable: write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("writeDurable: sync %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writeDurable: close %s: %w", path, err)
	}
	return nil
}

// syncDir fsyncs the directory containing path so renames/creates into it are durable.
// Best-effort: on platforms/filesystems where directory fsync is unsupported, the
// caller treats the error as non-fatal (review Critical #4, Task 8).
func syncDir(path string) error {
	dir := filepath.Dir(path)
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
