package vivaldi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupSnapshotDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIVALDI_SNAPSHOT_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	return dir
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello world", "hello_world"},
		{"a/b/c", "a_b_c"},
		{"../etc/passwd", ".._etc_passwd"},
		{"with spaces and #symbols!", "with_spaces_and__symbols_"},
		{"valid-name_1.0", "valid-name_1.0"},
	}
	for _, c := range cases {
		got := sanitizeFilename(c.in)
		if got != c.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeFilename_EmptyReturnsTimestamp(t *testing.T) {
	// Empty input falls back to current UTC timestamp (filename-safe).
	got := sanitizeFilename("")
	if !strings.HasPrefix(got, "20") || !strings.HasSuffix(got, "Z") {
		t.Errorf("expected ISO timestamp fallback, got %q", got)
	}
}

func TestSaveSnapshot_CreatesFile(t *testing.T) {
	setupSnapshotDir(t)

	res, err := SaveSnapshot(SaveSnapshotOption{Label: "test-snap-1"})
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if res.SnapshotID != "test-snap-1" {
		t.Errorf("expected snapshot_id=test-snap-1, got %s", res.SnapshotID)
	}
	if res.Bytes == 0 {
		t.Errorf("expected bytes > 0")
	}
	if res.Path == "" {
		t.Errorf("expected path")
	}

	// File must exist + be valid JSON
	data, err := os.ReadFile(filepath.Join(res.Path, "snapshot.json"))
	if err != nil {
		t.Fatalf("read snapshot.json: %v", err)
	}
	var rec SnapshotRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rec.SnapshotID != "test-snap-1" {
		t.Errorf("rec.SnapshotID mismatch: %s", rec.SnapshotID)
	}
	if rec.Version != "1.1.0" {
		t.Errorf("rec.Version mismatch: %s", rec.Version)
	}
}

func TestSaveSnapshot_AtomicWrite(t *testing.T) {
	// Atomicity guarantee: no .tmp-* file should remain after a save.
	dir := setupSnapshotDir(t)

	_, err := SaveSnapshot(SaveSnapshotOption{Label: "atomic-test"})
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// Walk the directory tree to find any leftover .tmp files.
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, ".tmp-") || strings.HasSuffix(name, ".tmp") {
			t.Errorf("tmp file leaked: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestListSnapshots_Empty(t *testing.T) {
	dir := setupSnapshotDir(t)

	res, err := ListSnapshots(0)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("expected empty, got %d snapshots", res.Count)
	}
	if res.BaseDir == "" {
		t.Errorf("expected base_dir")
	}
	_ = dir
}

func TestListSnapshots_OrderedByRecency(t *testing.T) {
	setupSnapshotDir(t)

	// Save three snapshots in order.
	for _, label := range []string{"first", "second", "third"} {
		if _, err := SaveSnapshot(SaveSnapshotOption{Label: label}); err != nil {
			t.Fatalf("SaveSnapshot %s: %v", label, err)
		}
	}

	res, err := ListSnapshots(0)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if res.Count != 3 {
		t.Fatalf("expected 3, got %d", res.Count)
	}
	// Newest first means 'third' > 'second' > 'first' lexically
	if res.Snapshots[0].SnapshotID != "third" {
		t.Errorf("expected third first, got %s", res.Snapshots[0].SnapshotID)
	}
	if res.Snapshots[1].SnapshotID != "second" {
		t.Errorf("expected second, got %s", res.Snapshots[1].SnapshotID)
	}
	if res.Snapshots[2].SnapshotID != "first" {
		t.Errorf("expected first last, got %s", res.Snapshots[2].SnapshotID)
	}
}

func TestListSnapshots_Limit(t *testing.T) {
	setupSnapshotDir(t)
	for _, label := range []string{"aaa", "bbb", "ccc", "ddd"} {
		if _, err := SaveSnapshot(SaveSnapshotOption{Label: label}); err != nil {
			t.Fatalf("SaveSnapshot %s: %v", label, err)
		}
	}
	res, err := ListSnapshots(2)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("expected 2 with limit, got %d", res.Count)
	}
}

func TestBuildWorkspaceTabs_EvenSplit(t *testing.T) {
	profile := &Profile{
		Path: "/tmp/fake",
		Workspaces: []Workspace{
			{ID: "w1", Name: "W1"},
			{ID: "w2", Name: "W2"},
			{ID: "w3", Name: "W3"},
		},
	}
	tabs := []TabSummary{}
	for i := 0; i < 10; i++ {
		tabs = append(tabs, TabSummary{URL: "https://example.com/" + string(rune('0'+i))})
	}
	out := buildWorkspaceTabs(profile, tabs, "")
	if len(out) != 3 {
		t.Fatalf("expected 3 workspaces, got %d", len(out))
	}
	// 10 tabs / 3 workspaces = 4,3,3 (or 4,3,3)
	total := 0
	for _, w := range out {
		total += w.Count
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
}

func TestBuildWorkspaceTabs_OnlyOne(t *testing.T) {
	profile := &Profile{
		Path: "/tmp/fake",
		Workspaces: []Workspace{
			{ID: "w1", Name: "W1"},
			{ID: "w2", Name: "W2"},
		},
	}
	tabs := []TabSummary{
		{URL: "https://a.com"},
		{URL: "https://b.com"},
	}
	out := buildWorkspaceTabs(profile, tabs, "w1")
	if len(out) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(out))
	}
	if out[0].Count != 2 {
		t.Errorf("expected 2 tabs in w1, got %d", out[0].Count)
	}
}

func TestBuildWorkspaceTabs_Empty(t *testing.T) {
	profile := &Profile{Path: "/tmp/fake"}
	if out := buildWorkspaceTabs(profile, nil, ""); len(out) != 0 {
		t.Errorf("expected empty, got %d", len(out))
	}
}
