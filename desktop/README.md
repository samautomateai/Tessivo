# Tessivo desktop (Flutter)

Native desktop UI for Tessivo: no Chromium or webview. Talks to the Go core's local JSON API
(`docs/api.md`, ADR 0004) with `dart:io` only: a Unix socket on macOS/Linux, loopback on Windows,
with the session token from `run/endpoint.json`. The app reuses a healthy core or starts
`tess serve --exit-with-parent` itself (the `tess` binary bundled next to the app, else on PATH),
and stops it on exit. If the core cannot start, the app falls back to `DemoCoreClient` sample data.

```sh
task desktop:run     # run on this OS (needs the OS toolchain; task doctor prints the steps)
task desktop:check   # pub get --enforce-lockfile, format check, analyze (fatal infos), tests with coverage ≥ 80%
```

**Chat** lists sessions (search, pin, rename, archive, delete) beside the open conversation. Replies
stream in and render Markdown (headings, lists, tables, code; no HTML, links or images are loaded);
**Stop** cancels the stream and the core keeps the partial reply. Each chat picks its model alias and
thinking effort. Attach files or folders by absolute path, or a notebook's context. **Memory** holds
facts every chat sees. **Save as output** restructures a reply into a validated Summarize, Analyze,
Document or Report output; **Make this repeatable** previews and creates a notebook from the chat.

In a notebook, **Add** takes an absolute file or folder path (checked before it is sent; there is no
file-picker package), and **Memory** keeps up to 50 short facts sent with every run. The outputs library
opens a saved output's Markdown. **Settings** shows the core's aliases, fallback chains, defaults,
limits and providers (key references only, never values or headers) and can list each provider's models;
edit them with `tess config`.

The **Templates** section is a gallery of built-in and user templates, filterable by work type.
"Use template" creates and opens a notebook (context slots to fill, schedule disabled); the selected
notebook can be saved as a user template, and user templates can be removed. The new-notebook dialog
links here.

The **Schedules** section lists the selected notebook's schedules (next run, last status, enable switch,
remove) and adds new ones (cron, missed-run policy, optional prompt). The app subscribes to `/v1/events`
and shows a dismissible banner when a scheduled run finishes, fails or is skipped (the core it starts also
shows an OS notification, via `tess serve --notify`); the stream reconnects
with backoff capped at one minute, and malformed or oversized frames are dropped.

Material widgets come from the standalone `material_ui` package (moved out of the Flutter
framework in 3.47). State uses `ChangeNotifier`; no state-management package.
