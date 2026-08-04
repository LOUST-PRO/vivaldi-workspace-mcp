// Package filex provides filesystem helpers for the vivaldi-workspace-mcp.
//
// Currently exposes WriteFileAtomic, which writes data to a temporary
// file in the same directory, fsyncs, then renames into place. This
// prevents corrupted output if the process is killed mid-write.
package filex

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path atomically: write to <path>.tmp,
// fsync, rename into place. On platforms where rename-over-existing is
// supported (Linux yes, Windows no), this is atomic. Mode is the final
// file mode bits.
//
// Returns the bytes written.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) (int, error) {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-")
	if err != nil {
		return 0, fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup on any error path. If the function returns
	// with err == nil, the temp file has been renamed and no longer
	// exists at tmpName.
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // ignored if already renamed
	}()

	if _, err := tmp.Write(data); err != nil {
		return 0, fmt.Errorf("write temp %s: %w", tmpName, err)
	}

	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("fsync temp %s: %w", tmpName, err)
	}

	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("close temp %s: %w", tmpName, err)
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		return 0, fmt.Errorf("chmod temp %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return 0, fmt.Errorf("rename temp %s -> %s: %w", tmpName, path, err)
	}

	return len(data), nil
}
