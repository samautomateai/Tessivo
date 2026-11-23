import 'dart:async';
import 'dart:convert';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../theme.dart';

/// "Edit": change the open output's fields in a form and save them as a new
/// version, without running the model again.
class EditOutputButton extends StatelessWidget {
  const EditOutputButton({super.key, required this.state, required this.enabled});

  final AppState state;
  final bool enabled;

  Future<void> _open(BuildContext context) async {
    final data = state.thread?.detail?.data ?? const {};
    if (data.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    final edited = await showDialog<Map<String, Object?>>(
      context: context,
      builder: (_) => OutputEditDialog(data: data),
    );
    if (edited == null) return;
    try {
      await state.editOutput(edited);
      messenger.showSnackBar(const SnackBar(content: Text('Saved as a new version')));
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  @override
  Widget build(BuildContext context) => TextButton.icon(
    key: const Key('edit-output'),
    onPressed: enabled && (state.thread?.detail?.data.isNotEmpty ?? false) ? () => unawaited(_open(context)) : null,
    icon: const Icon(Icons.edit_note, size: 18),
    label: const Text('Edit'),
  );
}

/// How a field is edited: plain text, one item per line, or JSON.
enum _Kind { text, lines, json }

class OutputEditDialog extends StatefulWidget {
  const OutputEditDialog({super.key, required this.data});

  final Map<String, Object?> data;

  @override
  State<OutputEditDialog> createState() => _OutputEditDialogState();
}

class _OutputEditDialogState extends State<OutputEditDialog> {
  late final Map<String, _Kind> _kinds = {for (final e in widget.data.entries) e.key: _kindOf(e.value)};
  late final Map<String, TextEditingController> _fields = {
    for (final e in widget.data.entries) e.key: TextEditingController(text: _textOf(_kinds[e.key]!, e.value)),
  };
  final Map<String, String> _errors = {};

  static _Kind _kindOf(Object? v) => switch (v) {
    String() => _Kind.text,
    List() when v.every((x) => x is String) => _Kind.lines,
    _ => _Kind.json,
  };

  static String _textOf(_Kind k, Object? v) => switch (k) {
    _Kind.text => v as String,
    _Kind.lines => (v as List).join('\n'),
    _Kind.json => const JsonEncoder.withIndent('  ').convert(v),
  };

  /// "key_points" → "Key points".
  static String _label(String key) {
    final words = key.replaceAll('_', ' ').trim();
    return words.isEmpty ? key : words[0].toUpperCase() + words.substring(1);
  }

  @override
  void dispose() {
    for (final c in _fields.values) {
      c.dispose();
    }
    super.dispose();
  }

  void _save() {
    final out = <String, Object?>{};
    final errors = <String, String>{};
    for (final MapEntry(:key, value: c) in _fields.entries) {
      switch (_kinds[key]!) {
        case _Kind.text:
          out[key] = c.text;
        case _Kind.lines:
          out[key] = [
            for (final l in c.text.split('\n'))
              if (l.trim().isNotEmpty) l.trim(),
          ];
        case _Kind.json:
          try {
            out[key] = jsonDecode(c.text);
          } on FormatException {
            errors[key] = 'Not valid JSON';
          }
      }
    }
    if (errors.isNotEmpty) {
      setState(() {
        _errors
          ..clear()
          ..addAll(errors);
      });
      return;
    }
    Navigator.of(context).pop(out);
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Edit output'),
    content: SizedBox(
      width: 640,
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Saved as a new version; the model is not called. Lists take one item per line.',
              style: TextStyle(color: Tokens.textMuted),
            ),
            const SizedBox(height: 12),
            for (final MapEntry(:key, value: c) in _fields.entries)
              Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: TextField(
                  key: Key('edit-field-$key'),
                  controller: c,
                  minLines: 1,
                  maxLines: _kinds[key] == _Kind.text ? 6 : 10,
                  style: _kinds[key] == _Kind.json ? const TextStyle(fontFamily: 'monospace', fontSize: 12) : null,
                  decoration: InputDecoration(
                    labelText: _label(key),
                    helperText: _kinds[key] == _Kind.json ? 'JSON' : null,
                    errorText: _errors[key],
                    border: const OutlineInputBorder(),
                  ),
                ),
              ),
          ],
        ),
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(key: const Key('edit-save'), onPressed: _save, child: const Text('Save version')),
    ],
  );
}
