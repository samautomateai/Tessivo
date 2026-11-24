# ADR 0009: desktop design system (Carbon in a VS Code workbench)

- Status: accepted (2026-11-24)
- Spec: `docs/superpowers/specs/2026-11-24-enterprise-design-system-design.md` (with the approved mockups)

## Context

The first desktop UI was built from Material defaults: large type, rounded cards, four columns on the notebook page,
and a demo core that filled every screen with sample data when the real core was missing. It looked unfinished and
hid real failures (a fresh install's core exited for lack of a config, and the demo data covered it up). Tessivo
needs a UI people can show at work: calm, dense, readable, and honest about what the core is doing.

## Decision

- **IBM Carbon** is the design language: its color tokens (Gray 100 dark, White light, plus a high-contrast theme),
  type scale (body 13/18 px for a desktop tool), spacing scale, buttons, tags, tabs, fields, data tables, tiles,
  content switchers, notices and modals. Tokens live in `desktop/lib/src/design` (`TvColors` as a theme extension,
  `TvType`, `TvSize`, `TvSpace`, `TvRadius`, `TvMotion`); components in `design/components`.
- **VS Code's workbench** is the layout, under a Carbon UI shell header: activity bar (64 px, icon + label), side
  bar (Ctrl B, 180–480 px, remembered), page, secondary panel (Ctrl J: the output thread on notebook pages, "In this
  chat" on chat pages), 22 px status bar (core, default model, today's tokens, next scheduled run, approvals).
  Below 1100 px the panel closes and below 860 px the side bar; their shortcuts still open them for the session.
- **Corners follow VS Code**: 0 for regions, rows and tabs; 2 px for buttons, inputs, tags and cards; 4 px for
  menus, dialogs and tooltips (the user's ruling during the build).
- **IBM Plex Sans and Mono** (OFL 1.1) are bundled; **Carbon icons** (Apache-2.0) are bundled SVGs drawn with
  `flutter_svg` 2.3.2. Each icon names the action it stands for (play for Run, renew for Run again, and so on).
- **Themes**: system, light, dark, high contrast; **zoom** 80–200 % (Ctrl = / Ctrl - / Ctrl 0) scales the whole UI
  by laying out at window size / zoom and painting it larger, so nothing is left unscaled and text stays vector
  sharp. Both are saved in `desktop.theme`, `desktop.zoom` and `desktop.layout` in the core config.
- **OS text size**: Flutter's Windows engine already reads `HKCU\Software\Microsoft\Accessibility\TextScaleFactor`
  and passes it as the text scale, so no OS-specific code was written; macOS and Linux pass theirs the same way.
- **No demo data in the app**: the fake core lives only in `desktop/test/support/fake_core` and a test fails if the
  app imports it. Without a core the app shows *Starting the Tessivo core…* or *Can't reach the Tessivo core* with
  Retry, Copy details and how to start it by hand; a lost connection shows *Reconnecting…* and retries with back-off.
  `tess serve` starts on the default config when there is no config file yet.
- **One rule keeps it so**: `design_rules_test` fails when a screen, widget or shell file uses a raw colour or font
  size instead of the tokens.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Microsoft Fluent 2 | Native to Windows, out of place on macOS and Linux; its Flutter package would replace most widgets |
| A home-made "modern SaaS" style | Not a published system to follow; the user asked for one followed thoroughly |
| Carbon's own UI shell alone | Its side nav suits web apps; VS Code's activity bar, side bar and panel fit a desktop work tool better |
| Rewrite the UI beside the old one and swap at the end | Throws away the tested behaviour (190 tests); all the risk lands at the swap |
| Our own SVG renderer or an icon font built in `tools/` | `flutter_svg` is the standard way; building one for 35 icons makes no sense |

## Consequences

- Fonts and icons add about 1 MB to the app (Plex 950 KB, icons 56 KB).
- Every screen is built from the same few components, so later screens are cheaper and consistent; the rules test
  stops drift.
- There are no golden (pixel) tests: layout tests check sizes, overflow at 200 % zoom on a 1366×768 window,
  truncation, and contrast through the accessibility guidelines; looks are reviewed by hand on each OS.
- Material widgets still sit under some components (text fields, dropdowns, dialogs) with a Carbon theme on top;
  input dialogs keep `AlertDialog` under the Carbon dialog theme, confirmations use `showTvModal`.
