import 'errors.dart';
import 'mcp_models.dart';

/// In-memory MCP servers for the demo core. Mixed into `DemoCoreClient`,
/// which also attaches them to notebooks.
mixin DemoMcp {
  final List<McpServer> demoMcpServers = [
    const McpServer(
      name: 'github',
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-github'],
      env: {'GITHUB_TOKEN': r'${secret:GITHUB_TOKEN}'},
    ),
  ];

  Future<List<McpServer>> listMcpServers() async => List.unmodifiable(demoMcpServers);

  Future<McpServer> addMcpServer(McpServer server) async {
    if (!RegExp(r'^[a-z0-9][a-z0-9_-]{0,31}$').hasMatch(server.name)) {
      throw const CoreException(400, 'names use a-z, 0-9, _ and - (at most 32)');
    }
    if (server.command.isEmpty == server.url.isEmpty) {
      throw const CoreException(400, 'give exactly one of a command or a URL');
    }
    if (demoMcpServers.any((s) => s.name == server.name)) {
      throw CoreException(409, 'mcp server "${server.name}" already exists');
    }
    demoMcpServers.add(server);
    return server;
  }

  Future<void> removeMcpServer(String name) async {
    final before = demoMcpServers.length;
    demoMcpServers.removeWhere((s) => s.name == name);
    if (demoMcpServers.length == before) throw CoreException(404, 'mcp server "$name" not found');
  }

  Future<McpTestResult> testMcpServer(String name) async {
    if (!demoMcpServers.any((s) => s.name == name)) throw CoreException(404, 'mcp server "$name" not found');
    return const McpTestResult(
      tools: [
        McpEntry(name: 'search', description: 'Search issues and pull requests'),
        McpEntry(name: 'get_file', description: 'Read a file from a repository'),
      ],
      resources: [McpEntry(name: 'README', uri: 'repo://payments/README.md')],
      prompts: [McpEntry(name: 'review', description: 'Review a pull request')],
    );
  }
}
