import 'package:tessivo_desktop/src/core/errors.dart';
import 'package:tessivo_desktop/src/core/models.dart';

/// In-memory saved prompts for the demo core, per notebook, with their history.
mixin DemoPrompts {
  final Map<String, Map<String, SavedPrompt>> _prompts = {};
  final Map<String, List<PromptVersion>> _history = {}; // notebook/name → earlier versions, oldest first

  Future<List<SavedPrompt>> listPrompts(String notebookId) async {
    final list = (_prompts[notebookId] ?? const {}).values.toList()..sort((a, b) => a.name.compareTo(b.name));
    return List.unmodifiable(list);
  }

  Future<SavedPrompt> savePrompt(String notebookId, SavedPrompt prompt) async {
    if (!RegExp(r'^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$').hasMatch(prompt.name)) {
      throw const CoreException(400, "prompt name: use letters, digits, '.', '_' or '-'");
    }
    if (prompt.text.trim().isEmpty) throw const CoreException(400, 'prompt text is required');
    final saved = _prompts[notebookId] ??= {};
    final old = saved[prompt.name];
    var version = 1;
    if (old != null) {
      if (old.text == prompt.text && old.chain == prompt.chain) return old;
      (_history['$notebookId/${prompt.name}'] ??= []).add(PromptVersion(version: old.version, text: old.text));
      version = old.version + 1;
    }
    return saved[prompt.name] = SavedPrompt(
      name: prompt.name,
      text: prompt.text,
      model: prompt.model,
      temperature: prompt.temperature,
      maxOutputTokens: prompt.maxOutputTokens,
      chain: prompt.chain,
      version: version,
    );
  }

  Future<void> deletePrompt(String notebookId, String name) async {
    if (_prompts[notebookId]?.remove(name) == null) throw CoreException(404, 'prompt $name not found');
    _history.remove('$notebookId/$name');
  }

  Future<List<PromptVersion>> promptVersions(String notebookId, String name) async {
    final cur = _prompts[notebookId]?[name];
    if (cur == null) return const [];
    return [PromptVersion(version: cur.version, text: cur.text), ...?_history['$notebookId/$name']?.reversed];
  }

  Future<SavedPrompt> restorePromptVersion(String notebookId, String name, int version) async {
    final v = (await promptVersions(notebookId, name)).where((v) => v.version == version).firstOrNull;
    if (v == null) throw CoreException(404, 'prompt $name has no version $version');
    return savePrompt(notebookId, SavedPrompt(name: name, text: v.text));
  }
}
