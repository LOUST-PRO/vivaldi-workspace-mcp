package vivaldi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// LaunchOptions configures one LaunchURLs invocation.
//
// Field defaults (zero-value safe):
//   - Binary: "vivaldi"
//   - Timeout: 10s
//   - Args: nil (only URLs are appended)
type LaunchOptions struct {
	Binary  string
	Timeout time.Duration
	Args    []string
}

// LaunchURLs validates the URLs, starts Vivaldi with them as positional
// args, and waits (with timeout) for the process to either exit cleanly
// or be in a "running and stable" state.
//
// Vivaldi's CLI semantics (verified 2026-07):
//   - `vivaldi <url1> <url2> ...` reuses the running instance and adds
//     each URL as a new tab. We do NOT pass --new-window by default because
//     the user almost always wants tabs in the active window.
//
// Returns a LaunchResult with separate Accepted / Rejected lists so
// callers can surface partial failures.
func LaunchURLs(urls []string, opts LaunchOptions) (LaunchResult, error) {
	start := time.Now()

	if opts.Binary == "" {
		opts.Binary = "vivaldi"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	// Validate path early — fail fast before spawning a process.
	binPath, err := exec.LookPath(opts.Binary)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("vivaldi binary not found in PATH (%q): %w", opts.Binary, err)
	}

	accepted, rejected := splitValidURLs(urls)
	if len(accepted) == 0 {
		return LaunchResult{
			Binary:        binPath,
			RequestedURLs: urls,
			RejectedURLs:  rejected,
			DurationMS:    time.Since(start).Milliseconds(),
		}, errors.New("no valid URLs to launch after validation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	args := append([]string{}, opts.Args...)
	args = append(args, accepted...)

	cmd := exec.CommandContext(ctx, binPath, args...)
	// Detach from the parent's stdout so MCP stdio is not polluted.
	// Vivaldi writes its own internal logs; we surface errors only.
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return LaunchResult{
			Binary:        binPath,
			RequestedURLs: urls,
			RejectedURLs:  rejected,
			DurationMS:    time.Since(start).Milliseconds(),
		}, fmt.Errorf("failed to start vivaldi: %w", err)
	}

	pid := cmd.Process.Pid

	// Wait briefly so we surface immediate failures (missing library,
	// invalid profile, etc.) but do not block the MCP client forever if
	// Vivaldi stays running.
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case err := <-waitDone:
		if err != nil {
			// Process exited non-zero. Could be: profile locked,
			// DISPLAY not set, locale issues. Surface the error.
			return LaunchResult{
				Binary:        binPath,
				PID:           pid,
				RequestedURLs: urls,
				LaunchedURLs:  nil,
				RejectedURLs:  rejected,
				DurationMS:    time.Since(start).Milliseconds(),
			}, fmt.Errorf("vivaldi exited non-zero: %w", err)
		}
	case <-ctx.Done():
		// Timeout: Vivaldi is still running (good — accepted URLs
		// are queued). Do NOT call cmd.Process.Kill(): we want it
		// to live and serve the queued URLs.
	}

	return LaunchResult{
		Binary:        binPath,
		PID:           pid,
		RequestedURLs: urls,
		LaunchedURLs:  accepted,
		RejectedURLs:  rejected,
		DurationMS:    time.Since(start).Milliseconds(),
	}, nil
}

// splitValidURLs returns (accepted, rejectedURLs) where accepted contains
// only URLs starting with http:// or https:// and longer than a trivial
// prefix. Empty tokens are summarized as a single "(empty)" entry so
// stray commas are reported but empty whitespace tokens are not counted
// multiple times.
func splitValidURLs(urls []string) (accepted []string, rejected []string) {
	emptyCount := 0
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			emptyCount++
			continue
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			rejected = append(rejected, u)
			continue
		}
		if len(u) < 12 {
			rejected = append(rejected, u)
			continue
		}
		accepted = append(accepted, u)
	}
	if emptyCount > 0 {
		rejected = append(rejected, fmt.Sprintf("(empty x %d)", emptyCount))
	}
	return accepted, rejected
}
