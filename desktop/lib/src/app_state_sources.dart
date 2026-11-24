part of 'app_state.dart';

/// Changes to the selected notebook's sources, memory and agent settings.
extension SourceActions on AppState {
  /// Adds a file or folder by absolute path to the selected notebook.
  Future<void> addContext(String path) async {
    final nb = selected;
    if (nb != null) _put(await core.addContext(nb.id, path));
  }

  /// Adds a web page to the selected notebook; it is fetched on every run.
  Future<void> addUrlContext(String url) async {
    final nb = selected;
    if (nb != null) _put(await core.addUrlContext(nb.id, url));
  }

  /// Saves pasted text as a source of the selected notebook.
  Future<void> addTextContext(String label, String text) async {
    final nb = selected;
    if (nb != null) _put(await core.addTextContext(nb.id, text, label: label));
  }

  /// Adds a repository's changes against [base] as a source, read with git on every run.
  Future<void> addGitDiffContext(String repo, String base) async {
    final nb = selected;
    if (nb != null) _put(await core.addGitDiffContext(nb.id, repo, base));
  }

  Future<void> refreshMcpServers() async {
    try {
      mcpServers = await core.listMcpServers();
    } on Object {
      mcpServers = const []; // an older core without MCP
    }
    if (!_disposed) _changed();
  }

  /// Sets the MCP servers the selected notebook's agent runs may use.
  /// Replaces the selected notebook's agent command allow-list.
  Future<void> setNotebookCommands(List<CommandRule> commands) async {
    final nb = selected;
    if (nb != null) _put(await core.setNotebookCommands(nb.id, commands));
  }

  Future<void> setNotebookMcp(List<String> servers) async {
    final nb = selected;
    if (nb != null) _put(await core.setNotebookMcp(nb.id, servers));
  }

  /// Adds an MCP resource to the selected notebook's sources.
  Future<void> addMcpContext(String server, String uri) async {
    final nb = selected;
    if (nb != null) _put(await core.addMcpContext(nb.id, server, uri));
  }

  /// Replaces the selected notebook's memory.
  Future<void> setMemory(List<String> memory) async {
    final nb = selected;
    if (nb != null) _put(await core.setMemory(nb.id, memory));
  }
}
