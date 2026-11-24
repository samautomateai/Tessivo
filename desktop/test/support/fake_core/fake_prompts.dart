import 'package:tessivo_desktop/src/core/errors.dart';
import 'package:tessivo_desktop/src/core/models.dart';

/// In-memory saved prompts for the demo core, per notebook.
mixin DemoPrompts {
  final Map<String, Map<String, SavedPrompt>> _prompts = {};

  Future<List<SavedPrompt>> listPrompts(String notebookId) async {
    final list = (_prompts[notebookId] ?? const {}).values.toList()..sort((a, b) => a.name.compareTo(b.name));
    return List.unmodifiable(list);
  }

  Future<SavedPrompt> savePrompt(String notebookId, SavedPrompt prompt) async {
    if (!RegExp(r'^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$').hasMatch(prompt.name)) {
      throw const CoreException(400, "prompt name: use letters, digits, '.', '_' or '-'");
    }
    if (prompt.text.trim().isEmpty) throw const CoreException(400, 'prompt text is required');
    return (_prompts[notebookId] ??= {})[prompt.name] = prompt;
  }

  Future<void> deletePrompt(String notebookId, String name) async {
    if (_prompts[notebookId]?.remove(name) == null) throw CoreException(404, 'prompt $name not found');
  }
}
