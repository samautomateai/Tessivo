# Tessivo desktop (Flutter)

Native desktop UI for Tessivo: no Chromium or webview. Talks to the Go core over local gRPC
(contract: `api/proto/tessivo/v1/tessivo.proto`). Until the gRPC client lands, the app runs on
`DemoCoreClient` with sample data so screens can be built and tested.

```sh
task desktop:init    # once: generate macOS, Windows and Linux runners (flutter create)
task desktop:run     # run on this OS
task desktop:check   # format check, analyze (fatal infos), tests with coverage ≥ 80%
```

Material widgets come from the standalone `material_ui` package (moved out of the Flutter
framework in 3.47). State uses `ChangeNotifier`; no state-management package.
