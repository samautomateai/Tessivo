import 'dart:async';
import 'dart:io';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/components/menu.dart';
import '../design/icons.dart';
import 'text_prompt_dialog.dart';

/// Saves the open output as Markdown or Word to a path the user confirms.
class ExportMenu extends StatelessWidget {
  const ExportMenu({
    super.key,
    required this.core,
    required this.notebookId,
    required this.outputId,
    required this.title,
    this.compact = false,
  });

  final CoreClient core;
  final String notebookId;
  final String outputId;
  final String title;
  final bool compact;

  /// The Downloads folder (or home) plus a file name made from the title.
  String _suggested(String ext) {
    final name = title.replaceAll(RegExp(r'[^\p{L}\p{N} ._-]+', unicode: true), ' ').trim();
    return downloadsPath('${name.isEmpty ? 'output' : name}.$ext');
  }

  Future<void> _export(BuildContext context, String format) async {
    final messenger = ScaffoldMessenger.of(context);
    final path = await askText(
      context,
      title: switch (format) {
        'docx' => 'Save as Word document',
        'pdf' => 'Save as PDF',
        _ => 'Save as Markdown',
      },
      label: 'File path',
      initial: _suggested(format),
      maxLength: 4096,
    );
    if (path == null || path.trim().isEmpty) return;
    try {
      final bytes = await core.exportOutput(notebookId, outputId, format);
      File(path.trim()).writeAsBytesSync(bytes, flush: true); // exports are small (≤ a few MB)
      messenger.showSnackBar(SnackBar(content: Text('Saved ${path.trim()}')));
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e is CoreException ? e.message : 'Could not save: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    void pick(String format) => unawaited(_export(context, format));
    return TvMenuButton(
      key: const Key('export-output'),
      label: 'Export',
      icon: TvIcon.export,
      compact: compact,
      items: [
        TvMenuItem('Word document (.docx)…', key: const Key('export-docx'), onSelected: () => pick('docx')),
        TvMenuItem('PDF (.pdf)…', key: const Key('export-pdf'), onSelected: () => pick('pdf')),
        TvMenuItem('Markdown (.md)…', key: const Key('export-md'), onSelected: () => pick('md')),
      ],
    );
  }
}

/// [fileName] in the user's Downloads folder (or home when there is none).
String downloadsPath(String fileName) {
  final env = Platform.environment;
  final home = env['USERPROFILE'] ?? env['HOME'] ?? Directory.systemTemp.path;
  final sep = Platform.pathSeparator;
  final downloads = Directory('$home${sep}Downloads');
  return '${downloads.existsSync() ? downloads.path : home}$sep$fileName';
}
