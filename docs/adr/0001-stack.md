# ADR 0001: Go core and CLI, Flutter desktop, no Chromium

- Status: accepted (2026-11-20); transport amended by [ADR 0004](0004-local-json-api.md)

## Context

Tessivo needs a CLI and a modern-looking native desktop app sharing one engine. Chromium and
webview-based shells (Electron, Wails, Tauri) are ruled out.

## Decision

- **Core and CLI in Go** (latest stable, pinned in `go.mod`). One engine; the CLI is a thin wrapper.
- **Desktop in Flutter** (stable channel). Own GPU renderer, no webview, runs on macOS, Windows, Linux.
- **Desktop ↔ core over a local API** (Unix socket / loopback, per-session token); JSON + SSE per ADR 0004. The core can also
  run headless for the CLI and, later, a daemon.

## Alternatives considered

Rust + GPUI (most native feel, but pre-1.0 API churn and thin docs), Gio/Fyne (pure Go, weaker
widget ecosystem), Wails/Tauri (webview; Chromium-based WebView2 on Windows).
