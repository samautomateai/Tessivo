import 'dart:async';
import 'dart:io';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import 'text_prompt_dialog.dart';

/// Saves the open output as Markdown or Word to a path the user confirms.
class ExportMenu extends StatelessWidget {
  const ExportMenu({
    super.key,
    required this.core,
    required this.notebookId,
    required this.outputId,
    required this.title,
  });

  final CoreClient core;
  final String notebookId;
  final String outputId;
  final String title;

  /// The Downloads folder (or home) plus a file name made from the title.
  String _suggested(String ext) {
    final env = Platform.environment;
    final home = env['USERPROFILE'] ?? env['HOME'] ?? Directory.systemTemp.path;
    final sep = Platform.pathSeparator;
    final downloads = Directory('$home${sep}Downloads');
    final dir = downloads.existsSync() ? downloads.path : home;
    final name = title.replaceAll(RegExp(r'[^\p{L}\p{N} ._-]+', unicode: true), ' ').trim();
    return '$dir$sep${name.isEmpty ? 'output' : name}.$ext';
  }

  Future<void> _export(BuildContext context, String format) async {
    final messenger = ScaffoldMessenger.of(context);
    final path = await askText(
      context,
      title: format == 'docx' ? 'Save as Word document' : 'Save as Markdown',
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
    return PopupMenuButton<String>(
      key: const Key('export-output'),
      tooltip: 'Export',
      icon: const Icon(Icons.file_download_outlined),
      onSelected: (f) => unawaited(_export(context, f)),
      itemBuilder: (_) => const [
        PopupMenuItem(key: Key('export-docx'), value: 'docx', child: Text('Word document (.docx)…')),
        PopupMenuItem(key: Key('export-md'), value: 'md', child: Text('Markdown (.md)…')),
      ],
    );
  }
}
