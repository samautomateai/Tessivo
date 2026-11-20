# Tessivo desktop (Flutter)

Native desktop UI for Tessivo: no Chromium or webview. Talks to the Go core's local JSON API
(`docs/api.md`, ADR 0004) with `dart:io` only: a Unix socket on macOS/Linux, loopback on Windows,
with the session token from `run/endpoint.json`. Start the core with `tess serve`; if it is not
running, the app falls back to `DemoCoreClient` sample data.

```sh
task desktop:init    # once: generate macOS, Windows and Linux runners (flutter create)
task desktop:run     # run on this OS
task desktop:check   # format check, analyze (fatal infos), tests with coverage ≥ 80%
```

Material widgets come from the standalone `material_ui` package (moved out of the Flutter
framework in 3.47). State uses `ChangeNotifier`; no state-management package.
