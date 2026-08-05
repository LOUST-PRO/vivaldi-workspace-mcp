# syntax=docker/dockerfile:1.7
#
# Multi-stage Dockerfile for vivaldi-workspace-mcp.
#
# Glama.ai's registry builds the image from this Dockerfile (or infers one
# from the repo if absent — see https://glama.ai/mcp/methodology). Keeping
# the build explicit here means Glama's introspection runs against the
# exact same artifact we ship, with no hidden AI-inference step.
#
# Image characteristics:
#   - Static binary (CGO_ENABLED=0, no glibc dependency) on Alpine 3.24.
#   - Runs as UID 65532 (the distroless-style "nonroot" convention).
#   - JSON-RPC frames travel on stdout, diagnostics on stderr. Do not
#     detach, redirect, or pipe — any I/O transformation will break the
#     MCP transport.
#   - No HEALTHCHECK because the binary speaks stdio-only JSON-RPC, not
#     HTTP. TCP probes do not apply; declaring HEALTHCHECK NONE makes the
#     absence intentional instead of implicit (per Docker docs, an image
#     without HEALTHCHECK is treated as "always healthy" by default).
#   - No VOLUME directive. All writable paths are caller-controlled via
#     CLI args (output_path, snapshot dir) or env vars (HOME, XDG_*).
#     Declaring a VOLUME would mount an anonymous volume that masks
#     those configuration knobs at runtime.
#   - No init wrapper (tini/dumb-init). The Go binary handles SIGINT
#     and SIGTERM natively via signal.NotifyContext in main.go; a
#     wrapper would add a process layer for zero benefit and could
#     break stdio passthrough if it buffered output.
#
# Caveats for the Glama introspection environment:
#   - This image does NOT ship a Vivaldi profile. The server starts and
#     responds to `initialize` / `tools/list` without one (the SDK
#     handles those before any tool handler runs). Individual tool calls
#     that touch the profile will return a structured ToolError envelope
#     rather than crashing; this is by design and matches the production
#     behavior on a host without Vivaldi installed.
#   - No network egress. The runtime never opens a socket, so behavioral
#     analysis of the container should report "no outbound traffic".

# ---- Stage 1: build --------------------------------------------------------
FROM golang:1.26.5-alpine3.24 AS builder

WORKDIR /src

# Pull modules first so they cache across source-only edits.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build flags rationale:
#   -trimpath        strips $GOPATH and absolute paths so the binary
#                    is reproducible across build hosts.
#   -ldflags '-s -w' drops the symbol and DWARF tables, reducing size.
#   CGO_ENABLED=0    produces a static binary so the runtime stage can
#                    be a minimal alpine without libc.
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
        -trimpath \
        -ldflags='-s -w' \
        -o /out/vivaldi-workspace-mcp \
        .

# ---- Stage 2: runtime ------------------------------------------------------
FROM alpine:3.24

# UID 65532 matches the "nonroot" convention used by distroless and
# most hardened container base images. -D creates without password
# (default shell becomes /sbin/nologin on Alpine).
RUN addgroup -g 65532 app \
    && adduser  -u 65532 -G app -D app

WORKDIR /app

COPY --from=builder /out/vivaldi-workspace-mcp /app/vivaldi-workspace-mcp

USER app:app

# Exec form (JSON array), not shell form. This is required for the
# binary to receive SIGINT/SIGTERM directly from the kernel — a shell
# wrapper would forward the signal to the shell, not the process.
ENTRYPOINT ["/app/vivaldi-workspace-mcp"]

# Explicitly disable HEALTHCHECK. The MCP transport is stdio JSON-RPC;
# there is no HTTP endpoint for a TCP probe to hit. Leaving HEALTHCHECK
# unset would inherit "always healthy" from the base image, but that
# inheritance is implicit and surprises some scanners. HEALTHCHECK NONE
# states the intent.
HEALTHCHECK NONE

# ---- Metadata --------------------------------------------------------------
LABEL org.opencontainers.image.title="vivaldi-workspace-mcp" \
      org.opencontainers.image.description="MCP server for inspecting, extracting, and managing Vivaldi browser workspaces and tab sessions on Linux." \
      org.opencontainers.image.source="https://github.com/LOUST-PRO/vivaldi-workspace-mcp" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="LOUST-PRO"