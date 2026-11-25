part of 'http_core_client.dart';

/// Saved prompt endpoints of the local API, with their version history (docs/api.md, "Prompts").
mixin _HttpPromptHistory on _HttpChat {
  String _promptPath(String notebookId, String name) =>
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/prompts/${Uri.encodeComponent(name)}';

  Future<List<SavedPrompt>> listPrompts(String notebookId) async => jsonList(
    (await _json('GET', '/v1/notebooks/${Uri.encodeComponent(notebookId)}/prompts'))['prompts'],
  ).map(SavedPrompt.fromJson).toList();

  Future<SavedPrompt> savePrompt(String notebookId, SavedPrompt prompt) async =>
      SavedPrompt.fromJson(await _json('PUT', _promptPath(notebookId, prompt.name), body: prompt.toBody()));

  Future<void> deletePrompt(String notebookId, String name) async {
    await _json('DELETE', _promptPath(notebookId, name));
  }

  Future<List<PromptVersion>> promptVersions(String notebookId, String name) async => jsonList(
    (await _json('GET', '${_promptPath(notebookId, name)}/versions'))['versions'],
  ).map(PromptVersion.fromJson).toList();

  Future<SavedPrompt> restorePromptVersion(String notebookId, String name, int version) async =>
      SavedPrompt.fromJson(await _json('POST', '${_promptPath(notebookId, name)}/versions/$version/restore'));
}
