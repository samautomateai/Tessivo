import 'dart:async';

import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/fields.dart';
import '../design/components/rows.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/theme.dart';
import '../design/type.dart';
import '../screens/new_notebook_dialog.dart';
import 'search_box.dart';

/// One palette command.
class PaletteCommand {
  const PaletteCommand(this.label, this.run);
  final String label;
  final Future<void> Function(BuildContext context) run;
}

/// Keyboard shortcuts for the whole window. Each binds Ctrl and Cmd, so they
/// work the same on every OS: K palette, N new notebook, comma settings,
/// = / − / 0 zoom, B side bar, J secondary panel.
class AppShortcuts extends StatelessWidget {
  const AppShortcuts({super.key, required this.state, required this.child});

  final AppState state;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    Map<ShortcutActivator, VoidCallback> both(LogicalKeyboardKey key, VoidCallback run) => {
      SingleActivator(key, control: true): run,
      SingleActivator(key, meta: true): run,
    };
    return CallbackShortcuts(
      bindings: {
        ...both(LogicalKeyboardKey.keyK, () => unawaited(showPalette(context, state))),
        ...both(LogicalKeyboardKey.keyN, () => unawaited(showNewNotebookDialog(context, state))),
        ...both(LogicalKeyboardKey.comma, () => state.go(Section.settings)),
        ...both(LogicalKeyboardKey.keyB, () => unawaited(state.toggleSideBar(width: MediaQuery.sizeOf(context).width))),
        ...both(LogicalKeyboardKey.keyJ, () => unawaited(state.togglePanel(width: MediaQuery.sizeOf(context).width))),
        ...both(LogicalKeyboardKey.equal, () => unawaited(state.stepZoom(1))),
        ...both(LogicalKeyboardKey.add, () => unawaited(state.stepZoom(1))),
        ...both(LogicalKeyboardKey.numpadAdd, () => unawaited(state.stepZoom(1))),
        ...both(LogicalKeyboardKey.minus, () => unawaited(state.stepZoom(-1))),
        ...both(LogicalKeyboardKey.numpadSubtract, () => unawaited(state.stepZoom(-1))),
        ...both(LogicalKeyboardKey.digit0, () => unawaited(state.stepZoom(0))),
        ...both(LogicalKeyboardKey.numpad0, () => unawaited(state.stepZoom(0))),
      },
      child: Focus(autofocus: true, child: child),
    );
  }
}

/// The commands the palette offers: sections, creating, every notebook.
List<PaletteCommand> paletteCommands(AppState state) => [
  for (final (s, label) in const [
    (Section.chat, 'Chat'),
    (Section.notebooks, 'Notebooks'),
    (Section.templates, 'Templates'),
    (Section.schedules, 'Schedules'),
    (Section.library, 'Outputs library'),
    (Section.settings, 'Settings'),
  ])
    PaletteCommand('Go to $label', (_) async => state.go(s)),
  PaletteCommand('New notebook', (context) => showNewNotebookDialog(context, state)),
  PaletteCommand('New chat', (_) async {
    state.go(Section.chat);
    await state.chat.newChat();
  }),
  PaletteCommand('Zoom in', (_) => state.stepZoom(1)),
  PaletteCommand('Zoom out', (_) => state.stepZoom(-1)),
  PaletteCommand('Reset zoom', (_) => state.stepZoom(0)),
  for (final (m, label) in const [
    (TvThemeMode.system, 'System'),
    (TvThemeMode.light, 'Light'),
    (TvThemeMode.dark, 'Dark'),
    (TvThemeMode.highContrast, 'High contrast'),
  ])
    PaletteCommand('Theme: $label', (_) => state.setThemeMode(m)),
  for (final nb in state.notebooks) PaletteCommand('Open notebook: ${nb.name}', (_) async => state.select(nb.id)),
  for (final r in state.research) PaletteCommand('Open research: ${r.name}', (_) async => state.selectResearch(r.id)),
];

/// Shows the palette; the chosen command runs after it closes.
Future<void> showPalette(BuildContext context, AppState state) async {
  final chosen = await showDialog<PaletteCommand>(
    context: context,
    builder: (_) => _Palette(commands: paletteCommands(state), state: state),
  );
  if (chosen != null && context.mounted) await chosen.run(context);
}

class _Palette extends StatefulWidget {
  const _Palette({required this.commands, required this.state});

  final List<PaletteCommand> commands;
  final AppState state;

  @override
  State<_Palette> createState() => _PaletteState();
}

class _PaletteState extends State<_Palette> {
  final _query = TextEditingController();

  @override
  void dispose() {
    _query.dispose();
    super.dispose();
  }

  List<PaletteCommand> get _matches {
    final words = _query.text.toLowerCase().split(RegExp(r'\s+')).where((w) => w.isNotEmpty).toList();
    final hits = [
      for (final c in widget.commands)
        if (words.every(c.label.toLowerCase().contains)) c,
    ];
    if (_query.text.trim().isNotEmpty) {
      final q = _query.text.trim();
      hits.add(PaletteCommand('Search for "$q"', (context) => runSearch(context, widget.state, q)));
    }
    return hits;
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final matches = _matches;
    return Dialog(
      key: const Key('palette'),
      alignment: Alignment.topCenter,
      insetPadding: const EdgeInsets.only(top: 80, left: 24, right: 24),
      child: SizedBox(
        width: 600,
        height: 420,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(TvSpace.s8),
              child: TextField(
                key: const Key('palette-input'),
                controller: _query,
                autofocus: true,
                textInputAction: TextInputAction.done,
                style: TvType.body.copyWith(color: c.textPrimary),
                cursorColor: c.interactive,
                decoration: tvFieldDecoration(c, hint: 'Type a command, a notebook or words to search').copyWith(
                  prefixIcon: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: TvSpace.s8),
                    child: TvIconView(TvIcon.search, color: c.textSecondary),
                  ),
                  prefixIconConstraints: const BoxConstraints(minWidth: 32),
                ),
                onChanged: (_) => setState(() {}),
                onSubmitted: (_) {
                  if (matches.isNotEmpty) Navigator.of(context).pop(matches.first);
                },
              ),
            ),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.only(bottom: TvSpace.s8),
                children: [
                  for (final (i, cmd) in matches.indexed)
                    TvTreeRow(
                      key: Key('palette-$i'),
                      leading: const SizedBox.shrink(),
                      label: cmd.label,
                      selected: i == 0,
                      onTap: () => Navigator.of(context).pop(cmd),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
