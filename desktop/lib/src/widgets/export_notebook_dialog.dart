import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/fields.dart';
import '../design/components/notices.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import 'export_menu.dart';
import 'text_prompt_dialog.dart';

/// Export…: where to save the archive and whether to include outputs; then
/// saves it and says where.
Future<void> exportNotebookDialog(BuildContext context, AppState state, Notebook nb) async {
  final got = await showDialog<(String, bool)>(
    context: context,
    builder: (_) => _ExportDialog(notebook: nb),
  );
  if (got == null || !context.mounted) return;
  final err = await state.exportNotebook(nb.id, got.$1, outputs: got.$2);
  if (!context.mounted) return;
  showTvToast(context, err ?? 'Saved ${got.$1}', kind: err == null ? TvNoticeKind.success : TvNoticeKind.error);
}

class _ExportDialog extends StatefulWidget {
  const _ExportDialog({required this.notebook});

  final Notebook notebook;

  @override
  State<_ExportDialog> createState() => _ExportDialogState();
}

class _ExportDialogState extends State<_ExportDialog> {
  late final _path = TextEditingController(text: downloadsPath(_fileName(widget.notebook.name)));
  bool _outputs = false;

  static String _fileName(String name) {
    final clean = name.replaceAll(RegExp(r'[^\p{L}\p{N} ._-]+', unicode: true), ' ').trim();
    return '${clean.isEmpty ? 'notebook' : clean}.tessivo.zip';
  }

  @override
  void dispose() {
    _path.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return AlertDialog(
      title: Text('Export "${widget.notebook.name}"'),
      content: SizedBox(
        width: 520,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'One file with the prompts, memory, copied sources and schedules. Run history, the audit log, '
              'conversations and keys never go in. Folders on this computer come along by name only.',
              style: TvType.body.copyWith(color: c.textSecondary),
            ),
            const SizedBox(height: TvSpace.s16),
            TvTextField(label: 'Save as', fieldKey: const Key('export-path'), controller: _path),
            const SizedBox(height: TvSpace.s12),
            TvCheckbox(
              key: const Key('export-outputs'),
              label: 'Include outputs',
              value: _outputs,
              onChanged: (v) => setState(() => _outputs = v),
            ),
          ],
        ),
      ),
      actions: [
        TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
        TvButton(
          'Export',
          key: const Key('export-save'),
          kind: TvButtonKind.primary,
          onPressed: () {
            final path = _path.text.trim();
            if (path.isNotEmpty) Navigator.of(context).pop((path, _outputs));
          },
        ),
      ],
    );
  }
}

/// Import…: asks for an archive path, then imports it as a notebook or a
/// research notebook.
Future<void> importArchiveDialog(BuildContext context, AppState state, {bool research = false}) async {
  final path = await askText(
    context,
    title: research ? 'Import a research notebook' : 'Import a notebook',
    label: 'Archive file (.tessivo.zip)',
    maxLength: 4096,
  );
  if (path == null || path.trim().isEmpty) return;
  unawaited(research ? state.importResearch(path.trim()) : state.importNotebook(path.trim()));
}
