# ADR 0007: tray mode in the desktop app

- Status: accepted (2026-11-23); approved OS-specific work (CLAUDE.md engineering rules)

## Context

Runs, agent jobs and schedules live in the core, which the desktop app starts and which stops with it. Closing the
window therefore stops scheduled work. Users who rely on schedules want the app to keep running out of sight, with
a way back in and a clear way to quit. Flutter has no tray or "intercept close" API; both need native code on each
OS.

## Decision

- A setting, `desktop.close_to_tray` in the config (off by default), shown in Settings → Desktop. It lives in the
  core config so it works the same on every OS without desktop-side paths.
- `lib/src/tray.dart` holds the logic behind an interface: `DesktopWindow` (init, hide, show, quit) and
  `TrayController`, which reads the setting when the window is closed and hides or quits. It is tested with a
  fake window.
- `lib/src/tray_plugins.dart` implements `DesktopWindow` with `tray_manager` 0.5.3 and `window_manager` 0.5.2
  (MIT). The tray menu has Open Tessivo and Quit Tessivo; clicking the icon opens the window.
- The only OS branch is the icon format: `.ico` on Windows, PNG elsewhere, as tray_manager requires.
- If the tray cannot be shown (for example a Linux desktop without an indicator area), the close button keeps its
  normal behaviour: "prevent close" is turned on only after the tray is up.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Run the core as an OS service (launchd, systemd, Windows service) | Per-OS installers and admin rights; much larger change. Still possible later for headless use (`tess serve`) |
| Our own platform channels per OS | Three native implementations to write and maintain; the plugins are small, MIT and widely used |
| tray_manager 0.9 / window_manager 0.7 | Released on 2026-11-22, a rewrite on the new `nativeapi` packages; too new to trust |

## Consequences

- Linux builds need `libayatana-appindicator3-dev` (added to the release workflow); users need the runtime library,
  which common desktops ship. GNOME needs the AppIndicator extension to show the icon.
- Building on Windows needs Developer Mode (Flutter plugins use symlinks); CI runners have it.
- The plugin class is not covered by widget tests (no plugin channels there); its logic is in `TrayController`.
