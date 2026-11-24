# Enterprise design system for the desktop app — design

- Status: approved direction (2026-11-24); this document awaits approval
- Mockups (approved): [layout and themes](assets/2026-11-24-design/layout-and-themes.html),
  [notebook and chat with tool calls](assets/2026-11-24-design/notebook-and-chat.html). They are fragments made for
  the brainstorming companion; open them in a browser for the layout, type and colours (the frame styles are missing).

## 1. Intent

Tessivo will be shown to enterprise buyers. Today the desktop app reads as a prototype ("AI slop"): four columns at
once, every section in a bordered card, rows of icon-and-label buttons with nothing primary, body text around 16 px,
and sample data that looks real. The goal is an app that looks like a serious, calm professional tool on first sight
and stays that way in daily use: dense where people work, quiet everywhere else, one obvious action per area.

Success: a stranger sees the notebook and chat screens and takes it for a mature enterprise product; nothing on screen
is made up; text follows the user's display scale.

## 2. Goals, non-goals, anti-goals

Goals

- One design system, followed everywhere: **IBM Carbon** for tokens (colour, type, spacing, motion) and components,
  shaped by **VS Code** for the workbench layout, density, keyboard use and themes.
- A new shell: Carbon header + VS Code workbench (section 5).
- Every screen moved onto it: notebook, chat, research, approvals, schedules, templates, library, settings, dialogs.
- **No demo data in the app.** The sample client becomes a test fake; the app shows real connection states and real
  empty states.
- Themes: Light, Dark, High contrast; System (follow the OS light/dark setting) is the default.
- Text scales with Windows: display scale (125 %, 150 %, …), the system text size where Flutter reports it, and an
  app zoom (Ctrl + / Ctrl − / Ctrl 0) that is remembered.

Non-goals

- New product features. Every screen keeps what it does today; only layout, look and wording change.
- A component library for other apps, a web version, a plugin/theme marketplace, user-made themes.
- Pixel-matching IBM's React components; we follow the Carbon guidelines with Flutter widgets.

Anti-goals (would be a failure even if it works)

- Decoration that says "AI": gradients, glow, sparkle icons, rounded pill buttons everywhere, emoji.
- Icons the user has to guess. An icon without a tooltip, or a rare action without a label.
- Something that looks right only at 100 % scale on the developer's screen.
- Losing behaviour: every existing test that checks behaviour must still pass (selectors may change).

## 3. Constraints

- Flutter stable, `material_ui` widgets underneath; no second UI framework. Dart format at 120 columns,
  `flutter analyze --fatal-infos`, desktop coverage ≥ 80 % (`task desktop:check`), files ≤ 500 lines.
- OS independent: the same look on Windows, macOS and Linux. Anything OS-specific needs approval first (CLAUDE.md).
- Dependencies need a reason in `docs/adr/0003-dependency-policy.md` and `THIRD_PARTY.md`.
- Licences: Carbon icons Apache-2.0, IBM Plex SIL Open Font License 1.1, `flutter_svg` MIT.
- Accessibility: WCAG 2.1 AA contrast in Light and Dark, AAA-style contrast in High contrast; every control has a
  semantic label; keyboard reaches everything; the existing `a11y_test.dart` guidelines keep passing.

## 4. Approaches considered

| Approach | Verdict |
| --- | --- |
| **A. Foundation first, then screen by screen** on one branch, merged when every screen is moved | **Chosen.** Tokens, type, icons and components are built once; the shell next; each screen then moves with its tests green. Old and new look coexist only on the branch. |
| B. Rewrite the UI beside the old one and swap at the end | Rejected: throws away tested behaviour (190 tests); all risk at the swap. |
| Microsoft Fluent 2 | Rejected by the user: native to Windows, out of place on macOS, and its Flutter package would replace most widgets. |
| A home-made "modern SaaS" style | Rejected by the user: not a published system to follow thoroughly. |
| Icons via our own SVG path parser, or an icon font built in `tools/` | Rejected: `flutter_svg` is the standard way; building a renderer or font tool for 40 icons makes no sense. |

## 5. Layout (decided, see mockups)

```
┌ Header 40 ─────────────────────────────────────────────────────────────────────────┐
│ ■ Tessivo │ Notebooks / Change risk review   [⌕ Search or run a command  Ctrl K]  ✓¹ 🔔 ⚙ │
├──────┬───────────────┬──────────────────────────────────────────────┬──────────────┤
│ Act. │ Side bar      │ Page header: title · tag · meta · [Agent] [▶ Run]           │
│ bar  │ (Ctrl B)      │ Tabs: Output │ Sources·3 │ Prompt │ Memory·2 │ Agent │ Runs   │
│ 64   │ 236           │ Toolbar: v2 title …  ↻ Run again ✎ Edit ⇄ Compare ⤓ Export │ Thread      │
│ icon │ Notebooks     │                                              │ (Ctrl J) 290 │
│ +    │ Outputs       │ Content (max reading width 760)              │              │
│ label│               │                                              │              │
├──────┴───────────────┴──────────────────────────────────────────────┴──────────────┤
│ Status bar 22: ● Core connected · smart → litellm/qwen3 · Today 412k / 2M · Next run · 1 approval │
└────────────────────────────────────────────────────────────────────────────────────┘
```

- **Header (Carbon UI shell, 40 px):** product mark, breadcrumbs, centred command/search field (opens the existing
  palette, Ctrl K), then Approvals (with count), Notifications, Settings as icon buttons with tooltips.
- **Activity bar (VS Code, 64 px):** Notebooks, Research, Chat, Schedules, Templates; Account at the bottom. Icon
  (20 px) with a 10 px label under it. The active item has a 2 px accent bar at its left.
- **Side bar (236 px, Ctrl B toggles):** the active section's list. Notebooks: notebooks (work-type dot), then the
  open notebook's outputs with version. Chat: Pinned / Today / Last week / Older. Research: research notebooks.
  Rows 26 px; selected row has a layer background and a 2 px accent at its left.
- **Main area:** page header (title 20/26, work-type tag, one line of meta, one primary action), Carbon tabs, a slim
  toolbar for the selected object (icon + label for actions, icon only for panel toggles), content.
  **Tabs replace the old four columns**: Sources, Prompt, Memory, Agent (MCP servers + command allow-list) and Runs
  each get the full width.
- **Secondary panel (290 px, Ctrl J toggles):** the output's thread on notebook pages; "In this chat" (context,
  tools, activity log) on chat pages.
- **Status bar (22 px, accent background):** core connection, default model, today's tokens against the limit,
  next scheduled run, approvals waiting. Items are clickable where they lead somewhere.
- Panels remember open/closed and width (drag to resize the side bar and secondary panel, 180–480 px).
- Below 1100 px wide the secondary panel closes; below 860 px the side bar closes; both reopen with their shortcut.

## 6. Design tokens

All values live in one place (`lib/src/design/`), exposed as a `ThemeExtension` so widgets never hard-code colours.

### 6.1 Colour — Carbon themes

| Token | Dark (Gray 100) | Light (White) | High contrast (dark) |
| --- | --- | --- | --- |
| background | #161616 | #ffffff | #000000 |
| layer-01 (side bars, panels, fields) | #262626 | #f4f4f4 | #000000 |
| layer-02 (secondary buttons, hover) | #393939 | #e0e0e0 | #1a1a1a |
| layer-selected | #353535 | #e0e0e0 | #0f4a85 |
| border-subtle (hairlines) | #2e2e2e | #e0e0e0 | #6fc3df |
| border-strong (table heads, cards) | #393939 | #c6c6c6 | #6fc3df |
| field-underline | #525252 | #c6c6c6 | #ffffff |
| text-primary | #f4f4f4 | #161616 | #ffffff |
| text-secondary | #c6c6c6 | #525252 | #ffffff |
| text-placeholder / helper | #8d8d8d | #6f6f6f | #d0d0d0 |
| interactive (focus, accents) | #4589ff | #0f62fe | #f38518 |
| button-primary | #0f62fe | #0f62fe | #0f62fe with 1 px white border |
| support-error / warning / success / info | #fa4d56 / #f1c21b / #42be65 / #4589ff | #da1e28 / #f1c21b / #24a148 / #0043ce | #ff8389 / #fddc69 / #6fdc8c / #a6c8ff |
| activity bar | #0d0d0d | #f4f4f4 | #000000 |
| status bar | #0f62fe, white text | #0f62fe, white text | #000000, 1 px #6fc3df top border |

Work-type tags (Carbon tag colours, readable in both themes):

| Work type | Dark tag (bg / text / dot) | Light tag (bg / text / dot) |
| --- | --- | --- |
| Analyze | #510224 / #ffafd2 / #ff7eb6 (magenta) | #ffd6e8 / #9f1853 / #d02670 |
| Summarize | #004144 / #9ef0f0 / #3ddbd9 (teal) | #9ef0f0 / #005d5d / #007d79 |
| Document | #002d9c / #d0e2ff / #78a9ff (blue) | #d0e2ff / #0043ce / #0f62fe |
| Report | #491d8b / #e8daff / #be95ff (purple) | #e8daff / #6929c4 / #8a3ffc |

High contrast uses the dark-theme tags with a 1 px border in the text colour.

### 6.2 Type — IBM Plex, VS Code density

Carbon's productive scale, one step smaller for body text to match VS Code's 13 px UI (the user found 14–16 px too
big). Fonts are bundled (IBM Plex Sans 400/500/600, IBM Plex Mono 400/500).

| Style | Size / line height | Weight | Letter spacing | Use |
| --- | --- | --- | --- | --- |
| page-title | 20 / 26 | 400 | 0 | Page header title, output title |
| heading-02 | 16 / 22 | 600 | 0 | Dialog titles, empty-state headings |
| heading-01 | 14 / 18 | 600 | 0.16 | Section headings inside content |
| body | 13 / 18 | 400 | 0.16 | All UI text, lists, tables, messages |
| body-long | 13 / 20 | 400 | 0.16 | Output prose and chat replies |
| label | 11 / 16 | 600, uppercase | 0.32 | Side-bar section labels |
| helper | 12 / 16 | 400 | 0.32 | Meta lines, helper text, status bar |
| caption | 10 / 12 | 500 | 0.2 | Activity-bar labels |
| code | 12 / 16 (Plex Mono) | 400 | 0 | Paths, commands, ids, tool output |

Reading content (output documents, chat turns) is capped at 760 px wide.

### 6.3 Spacing, size, shape, motion

- Spacing scale (Carbon): 2, 4, 8, 12, 16, 24, 32, 40, 48. Page padding 24 (header) / 32 (content).
- Heights: header 40, toolbar 38, tab bar 36, tree row 26, table row 36 (head 32), button 28 (24 in dense places),
  input 32, status bar 22.
- Corners (VS Code, user decision 2026-11-24: "not overly curved"): 0 for panels, rows, tabs and bars; 2 for
  buttons, inputs, tags, tool-call blocks and cards; 4 for menus, the palette, dialogs and tooltips; round only for
  avatars, status dots and count badges.
- Borders: 1 px hairlines between regions instead of cards. Cards only where a thing is a unit the user acts on
  (tool-call block, approval, suggestion).
- Elevation: none except menus, the palette and dialogs (one Carbon shadow).
- Motion: Carbon productive, 70 ms (hover, press) and 110 ms (panels, expand), standard easing; respect the OS
  "reduce motion" setting (`MediaQuery.disableAnimations`).
- Focus: 2 px `interactive` outline inset on every focusable control; never removed.

### 6.4 Icons — Carbon

- Carbon icons v11 (`@carbon/icons`, Apache-2.0), the 32 px artboard SVGs, vendored under `assets/icons/carbon/`
  with their licence; drawn at 16 px (20 px in the activity bar) with `flutter_svg`, tinted with the text colour.
- Only the icons used are shipped; one Dart file maps meaning → asset so screens never name files.
- Rules: every icon button has a tooltip (and a semantic label); actions used less than daily carry a text label
  next to the icon; the activity bar always shows labels.

| Meaning | Carbon icon |
| --- | --- |
| Notebooks / Research / Chat / Schedules / Templates | notebook / microscope / chat / time / template |
| Search · Approvals · Notifications · Settings · Account | search · task--approved · notification · settings · user--avatar |
| New (notebook, chat, source) · Close | add · close |
| Run · Run again · Edit · Compare · Export · New version | play · renew · edit · compare · download · version |
| Agent mode · Tools (MCP) · Command / terminal | bot · tool-kit · terminal |
| Output / document · Folder · Web page / link · Attach · Send | document · folder · link · attachment · send--alt |
| Side panel show / hide · Expand | side-panel--open / side-panel--close · chevron--right |
| Success · Warning · Error | checkmark--filled · warning--alt · error--filled (verify name) |

## 7. Components (built once, used everywhere)

`lib/src/design/` holds the tokens; `lib/src/design/components/` the shared widgets:

- **Buttons:** primary, secondary, ghost, danger; sizes 28 and 24; icon-only variant (tooltip required by its API).
- **Tag** (work type, status, small labels), **status dot**, **badge** (counts).
- **Tabs** (Carbon line tabs with counts), **tree row** (icon/dot, label, trailing meta, selected state).
- **Text field / text area** (Carbon field: layer background, bottom underline, label above, helper or error
  below), **dropdown**, **toggle**, **checkbox**.
- **Data table** (head row, 36 px rows, hairlines, optional zebra off) — used by outputs that hold tables, runs,
  schedules, settings lists.
- **Inline notification** (info, success, warning, error) and **toast** (replaces snack bars).
- **Empty state** (icon, heading-02, one sentence, one primary action).
- **Tool-call block** (one row: icon, tool, short args, tags such as *allow-listed* / *MCP* / *exit 1*, time;
  expands to output; terminal output in Plex Mono on #0d0d0d with red/green for failures/passes).
- **Approval card** (warning accent, tool and arguments, Allow once / Always in this chat / Deny).
- **Panel header** (label, actions), **page header** (title, tag, meta, primary action), **toolbar**.
- **Dialog / modal** (Carbon modal: title, body, footer buttons right; Esc closes), **menu**, **command palette**
  restyled.

The Markdown view (outputs, chat replies) is restyled with these tokens: headings, lists, tables, code blocks.

## 8. Scaling with Windows

- **Display scale** (125 %, 150 %, per monitor): Flutter applies it through the device pixel ratio; layout is in
  logical pixels, so it already follows. Verify on a 150 % monitor and when the window moves between monitors.
- **System text size** (Windows Settings → Accessibility → Text size): use `MediaQuery.textScalerOf(context)`
  everywhere text is measured; never the deprecated `textScaleFactor`. **First task of the plan: check whether
  Flutter on Windows reports this setting.** A forum report says it may not. If it does not, reading it needs
  Windows-only code (WinRT `UISettings.TextScaleFactor`), which needs the user's approval before it is written;
  until then the app zoom covers the need.
- **App zoom** (VS Code-style): Ctrl + / Ctrl − / Ctrl 0 (⌘ on macOS), 80 %–200 % in 10 % steps, applied as a
  `TextScaler` multiplied with the system one and as a layout scale for fixed sizes (bars, rows, icons). Remembered
  in the config (`desktop.zoom`, beside `close_to_tray`). Shown in Settings → Appearance and in the palette.
- Layout must hold at 200 % effective scale on a 1366×768 screen: panels collapse (section 5), text wraps or
  truncates with a tooltip, nothing overflows (tests check it).

## 9. Themes

- Settings → Appearance: Theme (System, Light, Dark, High contrast), Zoom. Saved as `desktop.theme`
  (`system` | `light` | `dark` | `high-contrast`) in the config, like `close_to_tray`.
- System follows the OS light/dark setting live (`MediaQuery.platformBrightness`). High contrast is also chosen
  automatically when the OS reports high contrast (`MediaQuery.highContrast`) and the setting is System.
- Theme switching is instant and needs no restart.

## 10. No demo data

- `DemoCoreClient` and the `demo_*.dart` files move to `test/support/` as the test fake; nothing in `lib/` refers
  to sample data. `main.dart` no longer falls back to demo data.
- **Connection states** (full-window, in the new shell):
  - *Starting the Tessivo core…* (spinner; the supervisor is starting `tess serve`).
  - *Can't reach the Tessivo core* — what happened (not installed next to the app / exited with this message /
    did not answer in time), **Retry**, **Copy details**, and how to start it by hand (`tess serve`). The last lines
    of the core's stderr are shown, redacted.
  - Lost connection while running: an inline warning bar and a status-bar state (*Reconnecting…*); the app retries
    with back-off and recovers without a restart.
- **First run / empty states** (no fake content):
  - No model set up: the existing setup banner becomes a first-run panel on the Notebooks page — *Connect a model*
    (opens Settings → Models) as the primary action, a one-line explanation.
  - No notebooks: empty state with *New notebook* and *Start from a template*.
  - Empty notebook tabs: Sources (*Add files, folders, web pages, git changes*), Output (*Run the prompt to get the
    first output*), Memory, Agent, Runs — each with one sentence and one action.
  - No chats, no research notebooks, no schedules, no approvals: same pattern.

## 11. Screen by screen

| Screen | Becomes |
| --- | --- |
| Notebook (workspace + sources pane + outputs pane + MCP/commands cards) | Notebook page: header, tabs (Output, Sources, Prompt, Memory, Agent, Runs), output toolbar, thread panel |
| Output thread, suggestions, compare, edit, rerun, export | Thread panel; suggestion as a card in the thread; compare and edit as Carbon modals; export as a toolbar menu |
| Chat (screen, dialogs, tools, approvals) | Chat page as mocked: list in the side bar, conversation, tool-call blocks, approval cards, composer, "In this chat" panel |
| Research (sources, chat, notes) | Research page: tabs (Sources, Ask, Notes), citations as inline links, sources side panel |
| Approvals sheet | Approvals page from the header icon: Carbon data table (run, tool, arguments, waiting since) with actions |
| Schedules | Schedules page: data table with next run, last result, enable toggle |
| Templates, library | Pages with Carbon tiles in a grid and a detail side panel |
| Settings (models, defaults, agent, MCP, desktop, diagnostics) | Settings page with a left list of sections (Models, Defaults and limits, Agent, MCP servers, Appearance, Desktop, Diagnostics) |
| Dialogs (new notebook, git changes, paste text, prompts, library, model check) | Carbon modals |
| Command palette | Restyled, opened from the header field and Ctrl K |

## 12. Testing and verification

- Behaviour tests keep passing; keys may be renamed only together with their tests.
- New tests: tokens per theme (contrast pairs meet AA/AAA), theme switching, zoom shortcuts and persistence, panel
  toggles and widths, connection states (starting, failed with details, reconnecting), each empty state, tool-call
  block and approval card, no overflow at 200 % on 1366×768 for the main pages.
- Golden images are **not** used (fonts render differently per OS in CI); layout is checked with widget tests.
- A manual review on Windows at 100 % and 150 % with Light, Dark and High contrast before merge, with screenshots
  attached to the pull request.

## 13. Docs and decisions

- ADR 0009 *Desktop design system* (Carbon + VS Code, IBM Plex, Carbon icons, `flutter_svg`, themes, zoom, no demo
  data).
- `docs/adr/0003-dependency-policy.md` and `THIRD_PARTY.md`: `flutter_svg`, IBM Plex, Carbon icons.
- `docs/user-guide.md`: layout, shortcuts (Ctrl K, Ctrl B, Ctrl J, zoom), themes; screenshots replaced.
- `docs/api.md`: `desktop.theme` and `desktop.zoom` in the settings body. `docs/checklist.md`: a row for the design
  system and one for removing demo data.

## 14. Calls made while writing this (check them)

- **Body text 13 px**, one step under Carbon's 14 px, to match VS Code and the user's "fonts are too big".
- **One high-contrast theme (dark)**, like VS Code's default high contrast; a light high-contrast theme can follow.
- **Zoom is applied** by multiplying the text scaler and every size token by the zoom factor from one inherited
  widget, not by transforming the whole canvas (which would blur text).
- **Developers without a running core** now see the connection screen instead of demo data, so `task desktop:run`
  builds `tess` first and puts it next to the app (as releases do).
- **Theme and zoom live in the core config** (`desktop.theme`, `desktop.zoom`) like `close_to_tray`, so they are the
  same for every window and survive reinstalls.
- **No golden-image tests** (fonts differ per OS in CI); widget tests check layout and overflow instead.

## 15. Left to the builder

- Exact component APIs and file split (respect the 500-line limit), as long as screens use components and tokens
  only (no raw colours or font sizes in screens; a test can scan `lib/src/screens` for `Color(0x` and `fontSize:`).
- Icon choices for meanings not in the table, from the Carbon set, following the same rules.
- Order of screen migration after the foundation and shell (suggested: notebook, chat, settings, research, the rest).
- Wording of empty states and errors, in the project's plain style.
