import 'package:material_ui/material_ui.dart';

/// Asks for one line of text; returns null on cancel or when left empty.
Future<String?> askText(
  BuildContext context, {
  required String title,
  String initial = '',
  String label = 'Name',
  int maxLength = 120,
}) async {
  final name = await showDialog<String>(
    context: context,
    builder: (_) => _TextDialog(title: title, initial: initial, label: label, maxLength: maxLength),
  );
  final trimmed = name?.trim();
  return trimmed == null || trimmed.isEmpty ? null : trimmed;
}

class _TextDialog extends StatefulWidget {
  const _TextDialog({required this.title, required this.initial, required this.label, required this.maxLength});

  final String title;
  final String initial;
  final String label;
  final int maxLength;

  @override
  State<_TextDialog> createState() => _TextDialogState();
}

class _TextDialogState extends State<_TextDialog> {
  late final _controller = TextEditingController(text: widget.initial);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: TextField(
        key: const Key('text-prompt'),
        controller: _controller,
        autofocus: true,
        maxLength: widget.maxLength,
        decoration: InputDecoration(labelText: widget.label),
        onSubmitted: (v) => Navigator.of(context).pop(v),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          key: const Key('text-prompt-ok'),
          onPressed: () => Navigator.of(context).pop(_controller.text),
          child: const Text('OK'),
        ),
      ],
    );
  }
}
