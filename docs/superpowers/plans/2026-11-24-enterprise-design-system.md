# Enterprise Design System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the desktop app's look on IBM Carbon tokens and components in a VS Code-shaped workbench, with
themes, zoom and Windows scaling, and no demo data in the shipped app.

**Architecture:** A `lib/src/design/` layer (tokens as a `ThemeExtension`, type, sizes, zoom, Carbon icons via
`flutter_svg`, shared components) is built first; a new workbench shell (`lib/src/shell/`) replaces `shell.dart`;
then each screen moves onto components only. The demo client becomes a test fake; the app shows connection and empty
states. Theme, zoom and panel layout are saved in the core config's `desktop` block.

**Tech Stack:** Flutter stable + `material_ui`, `flutter_svg` (new), IBM Plex (bundled fonts), Carbon icons
(vendored SVGs), Go core config (`internal/config`, `internal/service/settings.go`).

**Spec:** `docs/superpowers/specs/2026-11-24-enterprise-design-system-design.md` (mockups in
`docs/superpowers/specs/assets/2026-11-24-design/`).

## Global Constraints

- All colours, sizes and fonts come from `lib/src/design/`; screens and widgets contain no `Color(0x`, `fontSize:`
  or `Colors.` (enforced by `test/design_rules_test.dart`, Task 4).
- Type: page-title 20/26 400; heading-02 16/22 600; heading-01 14/18 600 (0.16); body 13/18 400 (0.16); body-long
  13/20; label 11/16 600 uppercase (0.32); helper 12/16 (0.32); caption 10/12 500 (0.2); code Plex Mono 12/16.
- Sizes: header 40, toolbar 38, tab bar 36, tree row 26, table row 36 (head 32), button 28 (dense 24), input 32,
  status bar 22, activity bar 64, side bar 236, secondary panel 290, reading width 760. Spacing 2/4/8/12/16/24/32/40/48.
- Corners (VS Code): `TvRadius.none` 0 (panels, rows, tabs, bars), `small` 2 (buttons, inputs, tags, tool blocks,
  cards), `medium` 4 (menus, palette, dialogs, tooltips); round only avatars, dots, count badges. Hairline borders
  instead of cards; focus outline 2 px interactive.
- Motion 70 ms / 110 ms; none when `MediaQuery.disableAnimations`.
- Colour values: spec section 6.1 (Dark Gray 100, Light White, High contrast), work-type tags as in 6.1.
- Every icon button has a tooltip and semantic label; less-than-daily actions show a text label; activity bar labels.
- Zoom 0.8–2.0 in 0.1 steps, Ctrl/⌘ + − 0; text uses `MediaQuery.textScalerOf`, never `textScaleFactor`.
- Config: `desktop.theme` ∈ {system, light, dark, high-contrast}, `desktop.zoom` 0.8–2.0, `desktop.layout`
  {side_bar_open, side_bar_width, panel_open, panel_width}, widths 180–480. Invalid values are refused on save and
  ignored (defaults used) on load by the desktop.
- Licences: Carbon icons Apache-2.0 (licence file shipped with the SVGs), IBM Plex OFL-1.1 (licence shipped),
  `flutter_svg` MIT; recorded in `docs/adr/0003-dependency-policy.md` and `THIRD_PARTY.md`.
- `task ci` green for every commit; Dart files ≤ 500 lines, tests ≤ 800; desktop coverage ≥ 80 %.
- No golden-image tests. No new product features (rulings below).

## Rulings (plan-level)

- **Runs tab dropped.** The spec lists tabs Output, Sources, Prompt, Memory, Agent, Runs, but the core has no API
  that lists a notebook's runs and the spec's non-goals forbid new features. Tabs are Output, Sources, Prompt,
  Memory, Agent; agent runs waiting for approval stay on the Approvals page. Cost if wrong: one tab and one API.
- **`task desktop:run` finds `tess` through `TESSIVO_TESS`.** `findTessExecutable` gains an explicit override
  (the environment variable `TESSIVO_TESS`), set by the Taskfile to the freshly built `bin/tess{{exeExt}}`; no
  OS branches. Cost if wrong: developers start `tess serve` by hand.

## Review Focus

- The core exits while the app is open: the app shows *Reconnecting…*, retries with back-off and recovers without a
  restart; it never shows sample content (Task 3).
- 200 % effective scale on 1366×768 (zoom 2.0 at 683×384 logical): notebook, chat and settings pages render without
  overflow; panels collapse (Tasks 9, 10, 12).
- Very long names (200-character notebook, chat and source names, deep breadcrumbs) truncate with an ellipsis and a
  tooltip instead of overflowing (Tasks 7, 9).
- A hand-edited config with `desktop.theme: "purple"` or `desktop.zoom: 9` does not break the app: the core refuses
  it on save, the desktop ignores it on load and uses System and 1.0 (Tasks 6, 8).
- High contrast: every foreground/background token pair used for text meets 7:1, Light and Dark 4.5:1 (Task 4).

---

### Task 1: Windows text-size check (decides whether OS-specific code is needed)

**Files:**
- Create: `desktop/lib/src/design/scale_probe.dart`
- Test: `desktop/test/scale_probe_test.dart`

**Interfaces:**
- Produces: `String describeScale(MediaQueryData mq)` → e.g. `"display 1.50 × text 1.25"`, shown later in
  Settings → Appearance (Task 8).

- [ ] **Step 1: Test** `describeScale formats display ratio and text scale`: `MediaQueryData(devicePixelRatio: 1.5,
  textScaler: TextScaler.linear(1.25))` → `'display 1.50 × text 1.25'`.
- [ ] **Step 2:** Run `flutter test test/scale_probe_test.dart` → FAIL (undefined).
- [ ] **Step 3:** Implement `describeScale` (uses `mq.textScaler.scale(1)`).
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Manual check on Windows** (record the result in the ledger): `flutter run -d windows` with a temporary
  `debugPrint(describeScale(MediaQuery.of(context)))` in `TessivoApp.build`; change Windows Settings →
  Accessibility → Text size to 125 % while the app runs; read the printed text scale. Remove the `debugPrint`.
  - If the text scale changes to 1.25: nothing more is needed; note it in ADR 0009 (Task 15).
  - If it stays 1.00: **stop and ask the user** whether to add Windows-only code that reads
    `UISettings.TextScaleFactor` (CLAUDE.md: OS-specific needs approval). Continue the other tasks meanwhile; zoom
    (Task 8) covers the need.
- [ ] **Step 6: Commit** `feat(desktop): describe the display and text scale`.

### Task 2: Demo client becomes a test fake

**Files:**
- Move: `desktop/lib/src/core/demo_*.dart` and the `DemoCoreClient` class (from `core_client.dart`) →
  `desktop/test/support/fake_core/` (one file per mixin, `fake_core.dart` holding `FakeCoreClient`).
- Modify: `desktop/lib/src/core/core_client.dart` (keeps `builtInWorkTypes` and the exports only),
  every test that imports `DemoCoreClient`.
- Test: `desktop/test/no_demo_data_test.dart`

**Interfaces:**
- Produces: `FakeCoreClient({Duration tick})` in `test/support/fake_core/fake_core.dart`, same behaviour as today's
  `DemoCoreClient`; `pumpApp` in `test/app_test.dart` uses it.

- [ ] **Step 1: Test** `no_demo_data_test.dart`: scans every file under `lib/` and fails if any contains
  `DemoCoreClient`, `demoOutputs`, `demoMcpServers`, `demoLibrary` or the sample names `'Change risk review'`,
  `'payments-service'`, `'Vendor contracts digest'`.
- [ ] **Step 2:** Run → FAIL (lib still has them).
- [ ] **Step 3:** Move the files (`git mv`), rename the class to `FakeCoreClient`, fix imports; `main.dart`'s
  `connectCore` keeps returning the client for now but no longer falls back (Task 3 finishes it): on failure it
  rethrows.
- [ ] **Step 4:** Run `flutter test` → all PASS; `flutter analyze --fatal-infos` clean; check coverage with
  `go run ../tools/covercheck -format lcov -profile coverage/lcov.info -min 80` after `flutter test --coverage`.
  If coverage drops under 80 %, add tests for the least-covered `lib/` files before committing.
- [ ] **Step 5: Commit** `refactor(desktop): demo data lives only in tests`.

### Task 3: Connection states (starting, failed, reconnecting)

**Files:**
- Create: `desktop/lib/src/connection.dart` (`CoreStatus`, `ConnectionGate` widget),
  `desktop/lib/src/screens/connection_screen.dart`
- Modify: `desktop/lib/main.dart`, `desktop/lib/src/core/supervisor.dart` (`TESSIVO_TESS`), `desktop/lib/src/app_state.dart`
  (connection-lost flag from the event stream), `Taskfile.yml` (`desktop:run` depends on `build`, sets `TESSIVO_TESS`)
- Test: `desktop/test/connection_test.dart`

**Interfaces:**
- Produces: `sealed class CoreStatus` with `CoreStarting`, `CoreFailed(String reason, String details)`,
  `CoreConnected(CoreClient client)`; `ConnectionGate({required Future<CoreClient> Function() connect,
  required Widget Function(CoreClient) builder})`; `AppState.connectionLost` (bool) and `AppState.reconnect()`;
  `findTessExecutable({String? resolvedExecutable, bool? isWindows, String? override})` (override wins when the file
  exists).

- [ ] **Step 1: Tests:**
  - `shows starting, then the app` — `connect` completes after a delay; first frame shows text
    `'Starting the Tessivo core…'`; after settle the builder's widget shows.
  - `shows why it failed with retry and details` — `connect` throws `CoreStartException('tess exited: bad config')`;
    screen shows `"Can't reach the Tessivo core"`, the reason, buttons `Retry` and `Copy details`, and the hint
    `tess serve`; tapping Retry calls `connect` again.
  - `reconnects after the core goes away` — fake event stream errors → `state.connectionLost == true`, an inline
    warning `Reconnecting to the Tessivo core…` and status bar text `Reconnecting…`; next successful poll clears it.
  - `TESSIVO_TESS overrides the bundled tess` — `findTessExecutable(override: <temp file>)` returns that path; a
    missing override falls back to today's rule.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement. Retry back-off for reconnect: 1, 2, 4, 8, 15 s then every 15 s; copy details uses
  `Clipboard.setData`; stderr lines shown are already redacted by the supervisor.
- [ ] **Step 4:** Run → PASS; `task desktop:run` starts the real core on this machine.
- [ ] **Step 5: Commit** `feat(desktop): real connection states instead of demo data`.

### Task 4: Tokens, type and themes

**Files:**
- Create: `desktop/lib/src/design/colors.dart` (`TvColors`), `type.dart` (`TvType`), `sizes.dart` (`TvSpace`,
  `TvSize`), `theme.dart` (`TvThemeMode`, `resolveColors`, `tvTheme`), `desktop/assets/fonts/` (IBM Plex Sans
  400/500/600, Plex Mono 400/500, `OFL.txt`)
- Modify: `desktop/pubspec.yaml` (fonts), `desktop/lib/src/theme.dart` (becomes a thin re-export, then removed in
  Task 14), `desktop/lib/main.dart` (`theme:`/`darkTheme:` from `tvTheme`)
- Test: `desktop/test/design_tokens_test.dart`, `desktop/test/design_rules_test.dart`

**Interfaces:**
- Produces: `class TvColors extends ThemeExtension<TvColors>` with fields `background, layer01, layer02,
  layerSelected, borderSubtle, borderStrong, fieldUnderline, textPrimary, textSecondary, textHelper, interactive,
  buttonPrimary, error, warning, success, info, activityBar, statusBar, statusBarText` and
  `TagColors tag(WorkTypeId)`; `static const TvColors dark, light, highContrast`; `TvColors.of(BuildContext)`.
  `abstract final class TvType` with `static TextStyle pageTitle, heading02, heading01, body, bodyLong, label,
  helper, caption, code` (family `IBMPlexSans` / `IBMPlexMono`). `enum TvThemeMode { system, light, dark,
  highContrast }`; `TvColors resolveColors(TvThemeMode mode, {required Brightness platform, required bool
  highContrast})`; `ThemeData tvTheme(TvColors c)`.

- [ ] **Step 1: Tests:**
  - `token values match the spec` — e.g. `TvColors.dark.background == Color(0xFF161616)`,
    `TvColors.light.interactive == Color(0xFF0F62FE)`, `TvColors.highContrast.borderSubtle == Color(0xFF6FC3DF)`,
    `TvColors.dark.tag(WorkTypeId.analyze).background == Color(0xFF510224)`.
  - `text pairs meet contrast` — for each theme, `contrast(textPrimary, background)`, `(textSecondary, layer01)`,
    `(textHelper, background)`: ≥ 4.5 in light/dark, ≥ 7 in high contrast (WCAG relative-luminance formula in the
    test); every tag's text on its background ≥ 4.5.
  - `type scale matches the spec` — `TvType.body.fontSize == 13 && height == 18/13`, `TvType.label` uppercase is
    applied by the widget (font 11, weight 600), `TvType.code.fontFamily == 'IBMPlexMono'`.
  - `resolveColors follows the OS` — system+dark → dark; system+light → light; system+highContrast → highContrast;
    explicit light ignores the OS.
  - `design_rules_test`: scans `lib/src/screens/` and `lib/src/widgets/` for `Color(0x`, `fontSize:`, `Colors.`;
    starts as a list of known offenders (today's files) that must only shrink, and is empty after Task 14.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; add fonts to `pubspec.yaml` (`family: IBMPlexSans`, weights 400/500/600;
  `IBMPlexMono` 400/500). `tvTheme` sets `ColorScheme` from the tokens, `fontFamily`, zero radii, `visualDensity`
  compact, and adds the extension.
- [ ] **Step 4:** Run → PASS; whole suite passes.
- [ ] **Step 5: Commit** `feat(desktop): Carbon tokens, IBM Plex type and three themes`.

### Task 5: Carbon icons

**Files:**
- Create: `desktop/assets/icons/carbon/*.svg` (32 px artboards from `@carbon/icons@11.53.0`, only those in the map,
  plus `LICENSE`), `desktop/lib/src/design/icons.dart`
- Modify: `desktop/pubspec.yaml` (`flutter_svg`, assets), `docs/adr/0003-dependency-policy.md`, `THIRD_PARTY.md`
- Test: `desktop/test/design_icons_test.dart`

**Interfaces:**
- Produces: `enum TvIcon` with values `notebooks, research, chat, schedules, templates, search, approvals,
  notifications, settings, account, add, close, run, runAgain, edit, compare, export, newVersion, agent, tools,
  terminal, document, folder, link, attach, send, panelOpen, panelClose, expand, success, warning, error` and
  `String get asset`; widget `TvIconView(TvIcon icon, {double size = 16, Color? color, String? semanticLabel})`
  (colour defaults to `DefaultTextStyle` colour; size multiplied by zoom from Task 8 once it exists).

- [ ] **Step 1: Tests:** `every icon has an asset listed in pubspec` (parse `pubspec.yaml` assets, check each
  `TvIcon.values[i].asset` file exists on disk); `icon renders in the text colour at 16 px` (pump `TvIconView`
  inside `DefaultTextStyle(color: red)`, find `SvgPicture`, check `colorFilter` and size).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Download each SVG from `https://unpkg.com/@carbon/icons@11.53.0/svg/32/<name>.svg` (names in the
  spec's table; verify `error--filled`), add `flutter_svg` (newest release at least two weeks old) with
  `flutter pub add`, implement.
- [ ] **Step 4:** Run → PASS; `pub get --enforce-lockfile` and `osv-scanner` clean.
- [ ] **Step 5: Commit** `feat(desktop): Carbon icons through flutter_svg`.

### Task 6: Theme, zoom and layout in the config (core)

**Files:**
- Modify: `internal/config/file.go` (`DesktopConfig`), `internal/config/config_test.go` or a new
  `internal/config/desktop_test.go`, `docs/api.md`
- Test: `internal/config/desktop_test.go`, `internal/service/settings_test.go`

**Interfaces:**
- Produces (Go): `DesktopConfig{CloseToTray bool; Theme string (json "theme"); Zoom float64 ("zoom");
  Layout *DesktopLayout ("layout")}`, `DesktopLayout{SideBarOpen *bool; SideBarWidth int; PanelOpen *bool;
  PanelWidth int}` (json snake_case), `func (d DesktopConfig) Validate() error` called from `File.Validate`.

- [ ] **Step 1: Tests:** valid: theme `""`, `system`, `light`, `dark`, `high-contrast`; zoom 0 (unset), 0.8, 2.0;
  widths 0, 180, 480. Invalid (error names the key): theme `purple`; zoom 0.5, 9; width 100, 900.
  `UpdateSettings` round-trips `desktop.theme` and keeps it when the block is absent (extend the existing desktop
  test).
- [ ] **Step 2:** `go test ./internal/config ./internal/service -run Desktop` → FAIL.
- [ ] **Step 3:** Implement; document the fields in `docs/api.md` (`PUT /v1/settings` row).
- [ ] **Step 4:** Run → PASS; `task lint`.
- [ ] **Step 5: Commit** `feat(config): desktop theme, zoom and layout`.

### Task 7: Components — basics

**Files:**
- Create: `desktop/lib/src/design/components/buttons.dart` (`TvButton`, `TvIconButton`), `tags.dart` (`TvTag`,
  `WorkTypeTag`, `TvBadge`, `TvStatusDot`), `tabs.dart` (`TvTabs`, `TvTabItem`), `rows.dart` (`TvTreeRow`),
  `headers.dart` (`TvPageHeader`, `TvPanelHeader`, `TvToolbar`), `notices.dart` (`TvInlineNotice`, `showTvToast`),
  `empty_state.dart` (`TvEmptyState`)
- Test: `desktop/test/components_basic_test.dart`

**Interfaces:**
- Produces: `TvButton(String label, {TvIcon? icon, VoidCallback? onPressed, TvButtonKind kind =
  TvButtonKind.secondary, bool dense = false})`, `enum TvButtonKind { primary, secondary, ghost, danger }`;
  `TvIconButton(TvIcon icon, {required String tooltip, VoidCallback? onPressed, int? badge})`;
  `TvTag(String text, {TvTagTone tone = TvTagTone.gray})`, `WorkTypeTag(WorkTypeId id)`, `TvBadge(int count)`;
  `TvTabs({required List<TvTabItem> tabs, required int selected, required ValueChanged<int> onSelect})`,
  `TvTabItem(String label, {int? count, Key? key})`; `TvTreeRow({required Widget leading, required String label,
  String? trailing, bool selected = false, VoidCallback? onTap, Key? key})` (label truncates with tooltip);
  `TvPageHeader({required String title, Widget? tag, String? meta, List<Widget> actions = const []})`;
  `TvPanelHeader(String label, {List<Widget> actions})`; `TvToolbar({Widget? leading, List<Widget> actions})`;
  `TvInlineNotice({required TvNoticeKind kind, required String title, String? message, Widget? action})`,
  `enum TvNoticeKind { info, success, warning, error }`, `void showTvToast(BuildContext, String message,
  {TvNoticeKind kind})`; `TvEmptyState({required TvIcon icon, required String title, required String message,
  String? actionLabel, VoidCallback? onAction})`.

- [ ] **Step 1: Tests:** heights from `TvSize` (button 28, dense 24, tree row 26, tab bar 36, toolbar 38); primary
  button uses `buttonPrimary`; `TvIconButton` exposes its tooltip as semantics label; `TvBadge(0)` renders nothing,
  `TvBadge(120)` shows `99+`; tree row with a 200-character label in a 236-px box has no overflow and a tooltip with
  the full text; `TvTabs` shows `Sources · 3` for count 3 and calls `onSelect`; each component renders in all three
  themes without exceptions; `meetsGuideline(labeledTapTargetGuideline)` for a row of icon buttons.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement with tokens only; hover/press states use `layer02` and 70 ms.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat(desktop): Carbon buttons, tags, tabs, rows, headers, notices`.

### Task 8: Zoom, theme switching and Settings → Appearance

**Files:**
- Create: `desktop/lib/src/design/zoom.dart` (`TvZoom`), `desktop/lib/src/widgets/settings_appearance.dart`
- Modify: `desktop/lib/main.dart` (theme from state), `desktop/lib/src/app_state.dart` (`themeMode`, `zoom`,
  load/save via settings), `desktop/lib/src/core/settings_draft.dart` (`theme`, `zoom`, `layout` accessors),
  `desktop/lib/src/widgets/command_palette.dart` (zoom and theme commands, shortcuts)
- Test: `desktop/test/appearance_test.dart`

**Interfaces:**
- Consumes: `TvThemeMode`, `resolveColors`, `tvTheme` (Task 4); Go `desktop.theme`/`zoom` (Task 6).
- Produces: `class TvZoom extends InheritedWidget { final double factor; double scale(double v); static TvZoom
  of(BuildContext) }` (missing ancestor → factor 1); `double nextZoom(double z, int direction)` (±0.1 clamped
  0.8–2.0, rounded to one decimal; direction 0 → 1.0); `AppState.themeMode`, `AppState.zoom`,
  `AppState.setThemeMode(TvThemeMode)`, `AppState.setZoom(double)` (save through `updateSettings` with the draft's
  `desktop` block); `SettingsDraft.theme` / `zoom` / `layout` getters and setters (unknown values read as defaults).

- [ ] **Step 1: Tests:** `nextZoom(1.0, 1) == 1.1`, `nextZoom(2.0, 1) == 2.0`, `nextZoom(0.8, -1) == 0.8`,
  `nextZoom(1.7, 0) == 1.0`; Ctrl+= / Ctrl+- / Ctrl+0 (and meta on macOS keys, both bound like the palette) change
  `state.zoom` and save it; `MediaQuery.textScalerOf` inside the app equals system scaler × zoom; theme menu
  (System/Light/Dark/High contrast) in Settings → Appearance switches `TvColors.of(context)` immediately; a config
  with `"theme":"purple","zoom":9` loads as System and 1.0; Appearance shows `describeScale` (Task 1).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; `TessivoApp` wraps `MaterialApp.builder` in `MediaQuery(textScaler: …)` and `TvZoom`;
  `TvSize` values used through `TvZoom.of(context).scale(...)` in components (update Task 7 components).
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat(desktop): themes and zoom that follow the system and are remembered`.

### Task 9: Workbench shell

**Files:**
- Create: `desktop/lib/src/shell/workbench.dart` (`Workbench`), `header.dart` (`TvHeader`), `activity_bar.dart`,
  `side_bar.dart` (resizable), `secondary_panel.dart`, `status_bar.dart`, `layout_prefs.dart` (`LayoutPrefs`)
- Modify: `desktop/lib/main.dart` (home = `Workbench`), `desktop/lib/src/app_state.dart` (`Section` adds
  `approvals`; `layout` prefs), remove `desktop/lib/src/shell.dart` once everything it rendered has a new home
- Test: `desktop/test/workbench_test.dart` (move shell assertions from `app_test.dart`)

**Interfaces:**
- Consumes: components (Task 7), icons (Task 5), zoom (Task 8), `CoreStatus`/`connectionLost` (Task 3).
- Produces: `Workbench({required AppState state})` with slots chosen by `state.section`:
  `Widget sideBarFor(Section)`, `Widget pageFor(Section)`, `Widget? panelFor(Section)` (pages register these in
  `lib/src/shell/sections.dart`); `class LayoutPrefs { bool sideBarOpen; double sideBarWidth; bool panelOpen;
  double panelWidth; }` with `fromDraft`/`toJson`; status bar items: connection, default model, today's tokens /
  limit, next scheduled run, approvals waiting (each a `TvStatusItem` with optional `onTap`).

- [ ] **Step 1: Tests:** header shows breadcrumbs `Notebooks / <name>` and the command field opens the palette;
  activity bar shows labels `Notebooks Research Chat Schedules Templates` and switches sections; Ctrl+B hides the
  side bar, Ctrl+J the secondary panel, both saved to `desktop.layout`; dragging the side-bar edge to 600 stops at
  480; at 1000 px wide the panel is closed and at 800 px the side bar too; the status bar shows
  `Core connected`, the default model, `Today 412 / 2000k tokens`-style text from usage, and `1 approval waiting`
  when there is one, which opens the Approvals page; at zoom 2.0 on a 1366×768 window nothing overflows.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; existing setup banner and notices move into `TvInlineNotice` under the header.
- [ ] **Step 4:** Run → PASS (all existing tests adapted to the new structure, keys kept where they exist).
- [ ] **Step 5: Commit** `feat(desktop): Carbon header and VS Code-style workbench`.

### Task 10: Notebook page

**Files:**
- Create: `desktop/lib/src/screens/notebook/notebook_page.dart`, `notebook_side_bar.dart`, `output_tab.dart`,
  `sources_tab.dart`, `prompt_tab.dart`, `memory_tab.dart`, `agent_tab.dart` (MCP servers + command allow-list),
  `thread_panel.dart`
- Modify/remove: `workspace_screen.dart`, `notebook_sources_pane.dart`, `notebook_outputs_pane.dart`,
  `notebook_mcp_card.dart`, `notebook_commands_card.dart`, `widgets/output_thread.dart`, `widgets/memory_card.dart`,
  `widgets/prompt_menu.dart`, buttons for rerun/compare/edit/export restyled
- Test: existing notebook tests adapted; new `desktop/test/notebook_page_layout_test.dart`

**Interfaces:**
- Consumes: shell slots (Task 9), components (Tasks 7, 11).
- Produces: `NotebookPage(state)`, `NotebookSideBar(state)`, `ThreadPanel(state)`; tab order Output, Sources,
  Prompt, Memory, Agent (ruling above).

- [ ] **Step 1: Tests:** page header shows title, `WorkTypeTag`, `≈ 51k tokens · <model>` and one primary `Run`;
  tabs show counts (`Sources · 3`, `Memory · 2`); output toolbar has `Run again`, `Edit`, `Compare` (only for
  revisions), `Export` with labels and a panel-toggle icon button with tooltip `Hide thread (Ctrl J)`; empty states:
  no output → `Run the prompt to get the first output`, no sources → `Add files, folders, web pages or git
  changes`; first run with no model set up → a panel `Connect a model` whose action opens Settings → Models; no
  notebooks → empty state with `New notebook` and `Start from a template`; a notebook named with 200 characters truncates in header, side bar and breadcrumbs; all behaviour tests
  from `notebook_page_test`, `url_context_test`, `output_edit_test`, `rerun_test`, `diff_test`, `export_test`,
  `prompts_test`, `prompt_library_test`, `notebook_commands_test`, `mcp_test` pass with updated finders.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement with components only; the Markdown view's styles come from `TvType` (body-long, code,
  heading-01) and tables use the data table (Task 11).
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat(desktop): notebook page on the workbench`.

### Task 11: Components — fields, table, modal, tool call, approval

**Files:**
- Create: `desktop/lib/src/design/components/fields.dart` (`TvTextField`, `TvDropdown`, `TvToggle`, `TvCheckbox`),
  `table.dart` (`TvDataTable`), `modal.dart` (`showTvModal`), `tool_call.dart` (`ToolCallBlock`), `approval.dart`
  (`ApprovalCard`)
- Test: `desktop/test/components_more_test.dart`

**Interfaces:**
- Produces: `TvTextField({TextEditingController? controller, String? label, String? helper, String? error, String?
  placeholder, int minLines = 1, int maxLines = 1, Key? key})`; `TvDropdown<T>({required List<(T, String)> items,
  required T value, required ValueChanged<T> onChanged, String? label})`; `TvToggle`, `TvCheckbox` (value,
  onChanged, label); `TvDataTable({required List<String> columns, required List<List<Widget>> rows, List<double?>?
  widths})`; `Future<T?> showTvModal<T>(BuildContext, {required String title, required Widget body, required
  List<Widget> actions})` (Esc closes, primary action last); `ToolCallBlock({required TvIcon icon, required String
  tool, String? args, List<TvTag> tags = const [], String? duration, String? output, bool initiallyExpanded =
  false})` (output in `TvType.code` on `activityBar` colour; lines starting `--- FAIL`/`FAIL` in `error`, `ok` in
  `success`); `ApprovalCard({required String tool, required String args, required VoidCallback onAllow, required
  VoidCallback onAlways, required VoidCallback onDeny})`.

- [ ] **Step 1: Tests:** field shows label above and error below in `error`; modal closes on Esc and returns the
  action's value; table head 32 and rows 36; tool block collapsed shows one 32-px row, expands on tap, colours a
  `--- FAIL` line with `error`; approval card buttons call their callbacks and have labels `Allow once`, `Always in
  this chat`, `Deny`.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(desktop): Carbon fields, table, modal, tool-call and approval components`.

(Task 11 may be done before Task 10 when Task 10 needs its components; keep the commit order fields → notebook.)

### Task 12: Chat page

**Files:**
- Create: `desktop/lib/src/screens/chat/chat_page.dart`, `chat_side_bar.dart` (Pinned / Today / Last week /
  Older), `conversation.dart`, `composer.dart`, `chat_context_panel.dart` ("In this chat": context, tools,
  activity)
- Modify/remove: `screens/chat_screen.dart`, `screens/chat_dialogs.dart`, `widgets/chat_widgets.dart`,
  `widgets/chat_tools.dart`
- Test: chat tests adapted; new `desktop/test/chat_page_layout_test.dart`

**Interfaces:**
- Consumes: `ToolCallBlock`, `ApprovalCard` (Task 11), shell slots (Task 9).
- Produces: `ChatPage(state)`, `ChatSideBar(state)`, `ChatContextPanel(state)`; `String chatGroup(DateTime created,
  DateTime now)` → `Today` / `Last week` / `Older`.

- [ ] **Step 1: Tests:** `chatGroup` boundaries (same day → Today, 6 days → Last week, 8 days → Older); a reply
  with tool activity shows one `ToolCallBlock` per call in order, `run_command` tagged `allow-listed` when the event
  says so; a waiting approval shows `ApprovalCard` in the conversation and in the panel's activity as `waiting`;
  composer shows attach, `Tools · n`, model, `≈ Nk tokens`, `Send`; at zoom 2.0 on 1366×768 no overflow; existing
  `chat_test`, `chat_tools_test`, `chat_http_test` pass with updated finders.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(desktop): chat page with tool calls and approvals in place`.

### Task 13: Settings page

**Files:**
- Create: `desktop/lib/src/screens/settings/settings_page.dart` (section list: Models, Defaults and limits, Agent,
  MCP servers, Appearance, Desktop, Diagnostics)
- Modify: `settings_screen.dart` (removed), `widgets/settings_*.dart`, `screens/mcp_settings.dart`,
  `widgets/model_check_dialog.dart` (as `showTvModal`)
- Test: settings tests adapted (`settings_edit_test`, `agent_settings_test`, `model_check_test`,
  `provider_presets_test`, `diagnostics_test`, `tray_test` toggle), new `settings_page_layout_test.dart`

**Interfaces:**
- Produces: `SettingsPage(state)`, `enum SettingsSection { models, defaults, agent, mcp, appearance, desktop,
  diagnostics }`; the side bar lists sections when the Settings section is active.

- [ ] **Step 1: Tests:** selecting each section shows its content; aliases as a `TvDataTable` with `Test` and
  `Edit…`; existing behaviour tests pass with updated finders.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(desktop): settings page with sections`.

### Task 14: Research, approvals, schedules, templates, library, dialogs, palette

**Files:**
- Modify/replace: `screens/research_screen.dart` (+ `widgets/research_*.dart`) → Research page with tabs Sources,
  Ask, Notes; `widgets/approvals_sheet.dart` → Approvals page (`TvDataTable`: run, tool, arguments, waiting since;
  Allow / Deny); `screens/schedules_screen.dart` → table with next run, last result, enable toggle;
  `screens/templates_screen.dart`, `screens/library_screen.dart` → tile grid + detail panel; every dialog
  (`new_notebook_dialog`, `git_diff_dialog`, `paste_text_dialog`, `text_prompt_dialog`, `prompt_library_dialog`,
  `output_edit_form`) → `showTvModal`; `command_palette.dart` restyled; `lib/src/theme.dart` removed
- Test: the existing tests for each (`research_screen_test`, `approvals_test`, `schedules_test`, `templates_test`,
  `workspace_extras_test`, `palette_test`, `search_test`, `a11y_test`) pass with updated finders; empty-state tests
  for no research notebooks, no approvals, no schedules, no templates; `design_rules_test` offender list is empty.
- [ ] **Step 1–4:** per screen: adapt tests → fail → implement → pass.
- [ ] **Step 5: Commit** one commit per screen group: `feat(desktop): research page`, `… approvals page`,
  `… schedules page`, `… templates and library`, `… dialogs and palette on Carbon`.

### Task 15: Docs, ADR and final checks

**Files:**
- Create: `docs/adr/0009-desktop-design-system.md`
- Modify: `docs/README.md` (ADR link), `docs/user-guide.md` (layout, shortcuts Ctrl K / Ctrl B / Ctrl J / zoom,
  themes, connection screen), `docs/checklist.md` (rows: design system; no demo data), `docs/architecture.md`
  (desktop layers: design, shell, screens), `README.md` (status line, screenshots), `THIRD_PARTY.md` (IBM Plex, Carbon
  icons, `flutter_svg`), `docs/adr/0003-dependency-policy.md`
- [ ] **Step 1:** Write ADR 0009: decision (Carbon + VS Code workbench, IBM Plex, Carbon icons via `flutter_svg`,
  themes, zoom, Windows text-size result from Task 1, no demo data), alternatives (Fluent 2, home-made, rewrite),
  consequences (fonts and icons add ~1.5 MB; no golden tests).
- [ ] **Step 2:** Update the other docs; `task text lengths`.
- [ ] **Step 3:** Run `task ci` → green; coverage ≥ 80 % Go and desktop.
- [ ] **Step 4: Manual review on Windows** at 100 % and 150 % display scale, zoom 1.0 and 2.0, each theme: notebook,
  chat, settings, research, empty states, connection-failed screen; save screenshots for the pull request.
- [ ] **Step 5: Commit** `docs: desktop design system (ADR 0009)`.
