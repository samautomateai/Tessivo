# Tessivo desktop (Flutter)

Native desktop UI for Tessivo: no Chromium or webview. Talks to the Go core's local JSON API
(`docs/api.md`, ADR 0004) with `dart:io` only: a Unix socket on macOS/Linux, loopback on Windows,
with the session token from `run/endpoint.json`. The app reuses a healthy core or starts
`tess serve --exit-with-parent` itself (the `tess` binary bundled next to the app, else on PATH),
and stops it on exit. If the core cannot start, the app falls back to `DemoCoreClient` sample data.

```sh
task desktop:init    # once: generate macOS, Windows and Linux runners (flutter create)
task desktop:run     # run on this OS
task desktop:check   # format check, analyze (fatal infos), tests with coverage ≥ 80%
```

Material widgets come from the standalone `material_ui` package (moved out of the Flutter
framework in 3.47). State uses `ChangeNotifier`; no state-management package.
