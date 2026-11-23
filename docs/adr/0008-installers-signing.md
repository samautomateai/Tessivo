# ADR 0008: installers and code signing

- Status: accepted (2026-11-23); approved OS-specific work (CLAUDE.md engineering rules)

## Context

Releases shipped archives only: unzip and run. Most people expect an installer that adds a menu entry and an
uninstaller, and an OS that does not warn about an unknown publisher. Each OS has its own installer format and its
own signing scheme, and both need the OS's tools.

## Decision

`go run ./tools/release installer -platform P` builds the installers from the staged bundle:

| OS | Installer | How |
| --- | --- | --- |
| Linux | `.deb` (app in `/opt/tessivo`, `tess` and `tessivo` in `/usr/bin`, menu entry, icon; depends on GTK 3 and libayatana-appindicator3) | Written in pure Go: an `ar` archive of `debian-binary`, `control.tar.gz`, `data.tar.gz`. Tested on every OS; CI also runs `dpkg-deb --info/--contents` |
| Linux | `.AppImage` | The tool stages an AppDir (bundle, `AppRun`, desktop entry, icon) and runs appimagetool 1.9.1, downloaded in CI with a pinned SHA-256 |
| Windows | `…_setup.exe` | The tool writes an Inno Setup script and runs `ISCC`. Per-user install by default (no admin), all users on request; Start menu entry, optional desktop shortcut, uninstaller, a fixed AppId for upgrades |
| macOS | `.dmg` | `Tessivo.app` and a link to `/Applications`, compressed with `hdiutil` |

The release workflow stages the bundle (`desktop -archive=false`), signs it, archives it (`desktop -stage=false`),
builds the installers and signs them. Signing runs only when the repository secrets exist, so forks and builds
without a certificate still produce unsigned artifacts:

| Secret | Use |
| --- | --- |
| `WINDOWS_CERT_PFX` (base64), `WINDOWS_CERT_PASSWORD` | Authenticode: imported into the runner's user store, `signtool /sha1 <thumbprint>` with an RFC 3161 timestamp; every `.exe`/`.dll` in the bundle, then `Setup.exe` |
| `MACOS_CERT_P12` (base64), `MACOS_CERT_PASSWORD`, `MACOS_SIGN_IDENTITY` | Developer ID Application: a throwaway keychain, `codesign --options runtime --timestamp` on `tess`, the app and the `.dmg` |
| `APPLE_API_KEY_P8` (base64), `APPLE_API_KEY_ID`, `APPLE_API_ISSUER` | Notarization with an App Store Connect API key (`notarytool submit --wait`), then `stapler staple` |

The macOS app is no longer sandboxed (`Release.entitlements`). It ships outside the App Store with a Developer ID,
where the sandbox is optional. The sandboxed build could not reach the core over loopback. It also could not let
`tess` read the folders the user picks, because a child process inherits the sandbox. Notarization only needs the
hardened runtime, which signing turns on.

## Alternatives considered

| Option | Why not |
| --- | --- |
| `dpkg-deb`, `fpm`, `nfpm` for the `.deb` | An OS tool or another dependency for a format that takes 200 lines of Go and can be tested anywhere |
| MSIX / WiX for Windows | MSIX needs Store-style packaging and signing for every install; WiX is heavier to author. Inno Setup is free, small and standard on the GitHub Windows runner (installed with Chocolatey when missing) |
| Flatpak / Snap | Their sandboxes conflict with reading arbitrary folders and starting `tess`; may come later |
| Passwords in `signtool /p` or `notarytool --password` | Puts secrets in argv; the certificate store and an API key file avoid that |

## Consequences

- The owner adds the secrets in the repository settings to get signed builds. Until then, Windows SmartScreen and
  macOS Gatekeeper warn on first start (the user guide explains how to open the app anyway).
- `security import -P` still takes the `.p12` password in argv on the macOS runner (no other way in `security`);
  the runner is ephemeral and GitHub masks the value in logs.
- The Windows AppId in `tools/release/installer.go` must never change, or upgrades install side by side.
- AppImage and `.deb` are built for amd64 only, like the Linux desktop job; arm64 needs an arm64 runner.
