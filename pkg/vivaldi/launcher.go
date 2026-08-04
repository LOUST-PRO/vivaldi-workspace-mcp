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
// or to remain running (which is the normal case for the running
// browser instance).
//
// Vivaldi CLI semantics (verified against Vivaldi 7.x on Linux, 2026-07):
//
//	$ vivaldi <url1> <url2> ...
//
// The above reuses the running Vivaldi instance if one is already
// listening on the user's session bus (single-instance lock at
// ~/.config/vivaldi/SingletonLock), and queues each URL as a new tab in
// the active window. If no instance is running, a new one is started
// with all URLs opened at once.
//
// We deliberately do NOT pass --new-window by default because most
// callers want tabs in the currently-active window, not a fresh one. If
// you need a new window per call, prepend it to opts.Args:
//
//	opts.Args = []string{"--new-window"}
//
// Vivaldi's CLI is a Chromium-style wrapper, so common Chromium flags
// pass through:
//
//	--profile-directory=NAME    use a non-"Default" profile
//	--user-data-dir=PATH        override ~/.config/vivaldi entirely
//	--disk-cache-dir=PATH       move cache off the default location
//	--no-first-run              skip the first-run welcome flow
//	--no-default-browser-check  skip the "make default?" prompt
//	--enable-logging=stderr     forward Chromium logs to stderr
//	--new-window                open URLs in a fresh window
//
// Useful environment variables:
//
//	DISPLAY                    must be set or the X server reachable
//	                           via xauth; without it, Vivaldi fails with
//	                           "Failed to connect to the bus".
//	XDG_RUNTIME_DIR            required for D-Bus / SingleInstanceLock.
//
// Returns a LaunchResult with separate Accepted / Rejected lists so
// callers can surface partial failures without losing visibility on
// what did get queued.
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

// splitValidURLs classifies a list of strings into URLs that Vivaldi
// will accept (http:// or https://, with a sane minimum length) and
// those it will reject.
//
// Why we filter:
//   - Vivaldi treats positional args that don't look like standard web
//     URLs (file://, javascript:, custom schemes) as either navigation
//     attempts or as flags, both of which can have surprising or unsafe
//     side effects. Restricting to http(s) keeps the launcher
//     predictable and prevents the most common prompt-injection footguns
//     (a model that emits "file:///etc/passwd" by mistake cannot
//     exfiltrate via this path).
//   - "http://x" alone (length 8) is too short to be a meaningful URL
//     and is almost always a typo or a stray fragment from a pasted
//     URL list.
//
// Empty tokens are summarized as a single "(empty x N)" entry so stray
// commas are reported but empty whitespace tokens are not counted
// multiple times in the rejected list.
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
