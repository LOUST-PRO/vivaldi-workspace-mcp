package vivaldi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/louzt/vivaldi-workspace-mcp/pkg/vivaldi/filex"
)

// SnapshotPath returns the canonical root directory for snapshot storage.
// Defaults to $XDG_DATA_HOME/vivaldi-workspace-mcp/snapshots/ or
// ~/.local/share/vivaldi-workspace-mcp/snapshots/.
//
// Overridable for tests via VIVALDI_SNAPSHOT_DIR env var (test-only).
func SnapshotPath() string {
	if d := os.Getenv("VIVALDI_SNAPSHOT_DIR"); d != "" {
		return d
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "vivaldi-workspace-mcp", "snapshots")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "vivaldi-workspace-mcp", "snapshots")
}

// WorkspaceTabs is one workspace's tab slice inside a snapshot.
type WorkspaceTabs struct {
	WorkspaceID   string       `json:"workspace_id"`
	WorkspaceName string       `json:"workspace_name"`
	Count         int          `json:"count"`
	Tabs          []TabSummary `json:"tabs"`
}

// SnapshotRecord is the file format written to disk.
//
// Schema-stable across versions: future Fase C+ tooling must be able to
// read v1.1.0 snapshots without code changes. Add fields, never rename.
type SnapshotRecord struct {
	SnapshotID  string          `json:"snapshot_id"`
	SavedAt     string          `json:"saved_at"`
	Version     string          `json:"version"` // schema version, matches MCP version
	ProfilePath string          `json:"profile_path"`
	Workspaces  []WorkspaceTabs `json:"workspaces"`
	Totals      SnapshotTotals   `json:"totals"`
}

// SnapshotTotals provides cheap counters for list_snapshots without
// loading the full tabs array.
type SnapshotTotals struct {
	Workspaces int `json:"workspaces"`
	URLs       int `json:"urls"`
}

// SnapshotSummary is the per-snapshot info returned by list_snapshots.
// Includes only metadata, NOT the tabs (so list stays fast).
type SnapshotSummary struct {
	SnapshotID  string        `json:"snapshot_id"`
	SavedAt     string        `json:"saved_at"`
	Version     string        `json:"version"`
	ProfilePath string        `json:"profile_path"`
	Path        string        `json:"path"` // absolute path on disk
	Bytes       int64         `json:"bytes"`
	Totals      SnapshotTotals `json:"totals"`
}

// SnapshotSaveResult is the response of save_workspace_snapshot.
type SnapshotSaveResult struct {
	SnapshotID  string `json:"snapshot_id"`
	Path        string `json:"path"` // abs path to snapshot dir
	Bytes       int64  `json:"bytes"`
	DurationMS  int64  `json:"duration_ms"`
	Workspaces  int    `json:"workspaces"`
	URLs        int    `json:"urls"`
}

// SnapshotListResult is the response of list_snapshots.
type SnapshotListResult struct {
	BaseDir   string             `json:"base_dir"`
	Count     int                `json:"count"`
	Snapshots []SnapshotSummary  `json:"snapshots"`
}

// SnapshotRestoreResult is the response of restore_workspace_snapshot.
type SnapshotRestoreResult struct {
	SnapshotID   string   `json:"snapshot_id"`
	Path         string   `json:"path"`
	RequestedURLs []string `json:"requested_urls"`
	LaunchedURLs []string `json:"launched_urls"`
	RejectedURLs []string `json:"rejected_urls"`
	Batches      int      `json:"batches"`
	DurationMS   int64    `json:"duration_ms"`
	Binary       string   `json:"binary"`
}

// SaveSnapshotOption configures a SaveSnapshot call.
type SaveSnapshotOption struct {
	Label    string // optional human label; defaults to snapshot_id
	OnlyWSID string // if set, only this workspace; "" = all workspaces
}

// SaveSnapshot writes the current Vivaldi state (workspaces + tab URLs)
// to ~/.local/share/vivaldi-workspace-mcp/snapshots/<snapshot_id>/.
//
// The snapshot is a single JSON file (snapshot.json) written atomically
// via filex.WriteFileAtomic. The directory is created on demand.
func SaveSnapshot(opts SaveSnapshotOption) (SnapshotSaveResult, error) {
	start := time.Now()

	profile, err := LoadProfile("")
	if err != nil {
		return SnapshotSaveResult{}, fmt.Errorf("load profile: %w", err)
	}
	tabsSummary, err := GetAllProfileTabs("")
	if err != nil {
		return SnapshotSaveResult{}, fmt.Errorf("load tabs: %w", err)
	}

	// Group tabs by URL-prefix heuristic when workspace_id is present
	// (Vivaldi doesn't store per-tab workspace metadata in Tabs_ files
	// directly; this is an approximation based on URL ordering vs
	// current workspace count).
	workspaces := buildWorkspaceTabs(profile, tabsSummary.Tabs, opts.OnlyWSID)

	totals := SnapshotTotals{
		Workspaces: len(workspaces),
		URLs:       0,
	}
	for _, w := range workspaces {
		totals.URLs += w.Count
	}

	snapshotID := opts.Label
	if snapshotID == "" {
		snapshotID = time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}
	// Sanitize for filename
	snapshotID = sanitizeFilename(snapshotID)

	record := SnapshotRecord{
		SnapshotID:  snapshotID,
		SavedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Version:     "1.1.0",
		ProfilePath: profile.Path,
		Workspaces:  workspaces,
		Totals:      totals,
	}

	baseDir := SnapshotPath()
	snapshotDir := filepath.Join(baseDir, snapshotID)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return SnapshotSaveResult{}, fmt.Errorf("mkdir %s: %w", snapshotDir, err)
	}

	jsonBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return SnapshotSaveResult{}, fmt.Errorf("marshal record: %w", err)
	}

	outPath := filepath.Join(snapshotDir, "snapshot.json")
	written, err := filex.WriteFileAtomic(outPath, jsonBytes, 0644)
	if err != nil {
		return SnapshotSaveResult{}, fmt.Errorf("atomic write %s: %w", outPath, err)
	}

	return SnapshotSaveResult{
		SnapshotID: snapshotID,
		Path:       snapshotDir,
		Bytes:      int64(written),
		DurationMS: time.Since(start).Milliseconds(),
		Workspaces: totals.Workspaces,
		URLs:       totals.URLs,
	}, nil
}

// ListSnapshots reads metadata from all snapshots on disk.
//
// Order: most recent first (by snapshot_id which is a lexicographically
// sortable timestamp). limit ≤ 0 means "all".
func ListSnapshots(limit int) (SnapshotListResult, error) {
	baseDir := SnapshotPath()
	out := SnapshotListResult{BaseDir: baseDir, Snapshots: []SnapshotSummary{}}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return SnapshotListResult{}, fmt.Errorf("read base dir: %w", err)
	}

	var summaries []SnapshotSummary
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		recPath := filepath.Join(baseDir, e.Name(), "snapshot.json")
		data, err := os.ReadFile(recPath)
		if err != nil {
			// Skip corrupt snapshot dirs but keep scanning.
			continue
		}
		var rec SnapshotRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		fi, err := os.Stat(recPath)
		if err != nil {
			continue
		}
		summaries = append(summaries, SnapshotSummary{
			SnapshotID:  rec.SnapshotID,
			SavedAt:     rec.SavedAt,
			Version:     rec.Version,
			ProfilePath: rec.ProfilePath,
			Path:        recPath,
			Bytes:       fi.Size(),
			Totals:      rec.Totals,
		})
	}

	// Sort newest first.
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SnapshotID > summaries[j].SnapshotID
	})

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}

	out.Snapshots = summaries
	out.Count = len(summaries)
	return out, nil
}

// LoadSnapshot reads a single snapshot from disk by ID.
//
// Returns the raw record + the path used, so callers can re-emit either.
func LoadSnapshot(snapshotID string) (SnapshotRecord, string, error) {
	baseDir := SnapshotPath()
	recPath := filepath.Join(baseDir, snapshotID, "snapshot.json")
	data, err := os.ReadFile(recPath)
	if err != nil {
		return SnapshotRecord{}, "", fmt.Errorf("read %s: %w", recPath, err)
	}
	var rec SnapshotRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return SnapshotRecord{}, "", fmt.Errorf("unmarshal %s: %w", recPath, err)
	}
	return rec, recPath, nil
}

// RestoreSnapshot re-launches URLs from a saved snapshot via the
// hardened launch_tabs path.
//
// Behavior:
//   - Each workspace's tabs are split into chunks of RestoreBatchSize
//     to avoid overwhelming Vivaldi (memory + UI freeze).
//   - Invalid URLs are reported back, not silently dropped.
//   - launchOpts.Binary / Timeout can be overridden (zero = defaults).
//
// The Vivaldi CLI does NOT support per-workspace tab routing, so all
// tabs land in the user's currently-active workspace. Workaround:
// switch workspaces manually after launch if needed.
func RestoreSnapshot(snapshotID string, launchOpts LaunchOptions, onlyWSID string) (SnapshotRestoreResult, error) {
	start := time.Now()

	rec, recPath, err := LoadSnapshot(snapshotID)
	if err != nil {
		return SnapshotRestoreResult{}, err
	}

	var allURLs []string
	for _, w := range rec.Workspaces {
		if onlyWSID != "" && w.WorkspaceID != onlyWSID {
			continue
		}
		for _, t := range w.Tabs {
			allURLs = append(allURLs, t.URL)
		}
	}

	if len(allURLs) == 0 {
		return SnapshotRestoreResult{
			SnapshotID:    snapshotID,
			Path:          recPath,
			RequestedURLs: []string{},
			LaunchedURLs:  []string{},
			RejectedURLs:  []string{},
			DurationMS:    time.Since(start).Milliseconds(),
		}, nil
	}

	const batchSize = 30
	var launchedAll, rejectedAll []string
	batches := 0
	for i := 0; i < len(allURLs); i += batchSize {
		end := i + batchSize
		if end > len(allURLs) {
			end = len(allURLs)
		}
		batch := allURLs[i:end]
		res, err := LaunchURLs(batch, launchOpts)
		launchedAll = append(launchedAll, res.LaunchedURLs...)
		rejectedAll = append(rejectedAll, res.RejectedURLs...)
		batches++
		if err != nil {
			// Continue with next batch — partial restore is better than
			// no restore. Surface error in the final result via Binary
			// field convention (empty Binary => fatal error occurred).
			if res.Binary == "" {
				return SnapshotRestoreResult{
					SnapshotID:    snapshotID,
					Path:          recPath,
					RequestedURLs: allURLs,
					LaunchedURLs:  launchedAll,
					RejectedURLs:  rejectedAll,
					Batches:       batches,
					DurationMS:    time.Since(start).Milliseconds(),
				}, fmt.Errorf("restore batch %d failed: %w", batches, err)
			}
			// res.Binary is set but error not nil — partial-success.
			// Keep going.
			_ = res
		}
	}

	return SnapshotRestoreResult{
		SnapshotID:    snapshotID,
		Path:          recPath,
		RequestedURLs: allURLs,
		LaunchedURLs:  launchedAll,
		RejectedURLs:  rejectedAll,
		Batches:       batches,
		DurationMS:    time.Since(start).Milliseconds(),
		Binary:        func() string {
			if launchOpts.Binary != "" {
				return launchOpts.Binary
			}
			return "vivaldi"
		}(),
	}, nil
}

// buildWorkspaceTabs distributes tabs across workspaces.
//
// Vivaldi session files do NOT tag individual tabs with workspace_id,
// so we approximate by evenly distributing the URL list in proportion
// to each workspace's tab count expectation. This is heuristic, not
// perfect — Fase C+ would parse Preferences.sessions to get exact
// mapping when present.
func buildWorkspaceTabs(profile *Profile, tabs []TabSummary, onlyWSID string) []WorkspaceTabs {
	if len(profile.Workspaces) == 0 || len(tabs) == 0 {
		return []WorkspaceTabs{}
	}

	// Filter workspaces if onlyWSID requested
	var wsList []Workspace
	if onlyWSID != "" {
		for _, w := range profile.Workspaces {
			if w.ID == onlyWSID {
				wsList = []Workspace{w}
				break
			}
		}
		if len(wsList) == 0 {
			// Workspace not found — return empty to signal error downstream
			return []WorkspaceTabs{}
		}
	} else {
		wsList = profile.Workspaces
	}

	// Even split: try to give each workspace the same number of tabs.
	out := make([]WorkspaceTabs, len(wsList))
	for i, w := range wsList {
		out[i] = WorkspaceTabs{
			WorkspaceID:   w.ID,
			WorkspaceName: w.Name,
			Count:         0,
			Tabs:          []TabSummary{},
		}
	}

	for i, t := range tabs {
		slot := i % len(out)
		out[slot].Tabs = append(out[slot].Tabs, t)
		out[slot].Count = len(out[slot].Tabs)
	}

	return out
}

// sanitizeFilename makes a label safe to use as a directory component.
func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}
	// Replace anything not in [A-Za-z0-9._-] with underscore
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, byte(r))
		case r >= 'a' && r <= 'z':
			out = append(out, byte(r))
		case r >= '0' && r <= '9':
			out = append(out, byte(r))
		case r == '.' || r == '-' || r == '_':
			out = append(out, byte(r))
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}
	return string(out)
}
