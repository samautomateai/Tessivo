part of 'http_core_client.dart';

/// MCP endpoints of the local API (docs/api.md, "MCP servers").
mixin _HttpMcp on _HttpChat {
  String _mcpPath(String name) => '/v1/mcp/servers/${Uri.encodeComponent(name)}';

  Future<List<McpServer>> listMcpServers() async {
    final json = await _json('GET', '/v1/mcp/servers');
    return jsonList(json['servers']).map(mcpServerFromJson).toList();
  }

  Future<McpServer> addMcpServer(McpServer server) async =>
      mcpServerFromJson(await _json('POST', '/v1/mcp/servers', body: server.toJson()));

  Future<void> removeMcpServer(String name) async {
    await _json('DELETE', _mcpPath(name));
  }

  Future<McpTestResult> testMcpServer(String name) async =>
      mcpTestFromJson(await _json('POST', '${_mcpPath(name)}/test'));

  Future<Notebook> setNotebookMcp(String notebookId, List<String> servers) async => notebookFromJson(
    await _json('PUT', '/v1/notebooks/${Uri.encodeComponent(notebookId)}/mcp', body: {'servers': servers}),
  );

  Future<Notebook> addMcpContext(String notebookId, String server, String uri) async => notebookFromJson(
    await _json(
      'POST',
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/context',
      body: {'server': server, 'uri': uri},
    ),
  );
}
