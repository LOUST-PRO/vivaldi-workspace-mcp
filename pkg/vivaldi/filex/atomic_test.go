package filex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "report.html")

	written, err := WriteFileAtomic(target, []byte("hello world"), 0644)
	if err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if written != 11 {
		t.Errorf("expected 11 bytes written, got %d", written)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}
	// Verify mode
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Mode().Perm() != 0644 {
		t.Errorf("expected mode 0644, got %v", fi.Mode().Perm())
	}
}

func TestWriteFileAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "report.html")

	if _, err := WriteFileAtomic(target, []byte("v1"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic v1: %v", err)
	}
	if _, err := WriteFileAtomic(target, []byte("v2 longer"), 0600); err != nil {
		t.Fatalf("WriteFileAtomic v2: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "v2 longer" {
		t.Errorf("expected %q, got %q", "v2 longer", got)
	}
}

func TestWriteFileAtomic_NoTempLeftover(t *testing.T) {
	// Atomicity guarantee: no .tmp-* file should remain after a successful
	// write. This prevents "ghost" temp files bloating the profile dir.
	dir := t.TempDir()
	target := filepath.Join(dir, "snapshot.json")
	if _, err := WriteFileAtomic(target, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}

func TestWriteFileAtomic_BadPath(t *testing.T) {
	// Path that points at a directory, not a file. Should fail.
	dir := t.TempDir()
	_, err := WriteFileAtomic(dir, []byte("x"), 0644)
	if err == nil {
		t.Fatalf("expected error when target is a directory")
	}
}
