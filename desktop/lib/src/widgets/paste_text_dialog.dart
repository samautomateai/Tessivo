import 'package:material_ui/material_ui.dart';

/// Asks for a label and pasted text; returns (label, text) or null.
class PasteTextDialog extends StatefulWidget {
  const PasteTextDialog({super.key});

  @override
  State<PasteTextDialog> createState() => _PasteTextDialogState();
}

class _PasteTextDialogState extends State<PasteTextDialog> {
  final _label = TextEditingController();
  final _text = TextEditingController();

  @override
  void dispose() {
    _label.dispose();
    _text.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Paste text as a source'),
    content: SizedBox(
      width: 620,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            key: const Key('paste-label'),
            controller: _label,
            maxLength: 200,
            decoration: const InputDecoration(labelText: 'Label', hintText: 'Pasted text'),
          ),
          TextField(
            key: const Key('paste-text'),
            controller: _text,
            minLines: 6,
            maxLines: 16,
            decoration: const InputDecoration(labelText: 'Text', border: OutlineInputBorder()),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('paste-save'),
        onPressed: () => Navigator.of(context).pop((_label.text.trim(), _text.text)),
        child: const Text('Add'),
      ),
    ],
  );
}
