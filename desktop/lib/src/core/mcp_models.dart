/// An MCP server as configured in the core (docs/api.md, "MCP servers").
/// Env and header values are stored text, which may hold ${secret:NAME}
/// placeholders; credentials themselves are never shown or sent.
class McpServer {
  const McpServer({
    required this.name,
    this.command = '',
    this.args = const [],
    this.url = '',
    this.env = const {},
    this.headers = const {},
    this.allowInsecureHttp = false,
    this.pack = '',
    this.trusted = true,
    this.mark = '',
  });

  final String name;
  final String command;
  final List<String> args;
  final String url;
  final Map<String, String> env;
  final Map<String, String> headers;
  final bool allowInsecureHttp;

  /// The team pack it comes from ('' for your own); its name is `<pack>/<server>`.
  final String pack;

  /// Your own servers always; a pack server only once you trusted it.
  final bool trusted;

  /// required or recommended by its pack.
  final String mark;

  String get transport => url.isEmpty ? 'stdio' : 'http';
  String get target => url.isEmpty ? [command, ...args].join(' ') : url;

  Map<String, Object?> toJson() => {
    'name': name,
    if (command.isNotEmpty) 'command': command,
    if (args.isNotEmpty) 'args': args,
    if (url.isNotEmpty) 'url': url,
    if (env.isNotEmpty) 'env': env,
    if (headers.isNotEmpty) 'headers': headers,
    if (allowInsecureHttp) 'allow_insecure_http': true,
  };
}

/// One tool, resource or prompt a server offers.
class McpEntry {
  const McpEntry({required this.name, this.description = '', this.uri = ''});
  final String name;
  final String description;
  final String uri; // resources only
}

/// What `test` found on a server.
class McpTestResult {
  const McpTestResult({required this.tools, required this.resources, required this.prompts});
  final List<McpEntry> tools;
  final List<McpEntry> resources;
  final List<McpEntry> prompts;
}

/// Parses `KEY=VALUE` lines; blank lines are skipped. Throws FormatException.
Map<String, String> parseKeyValues(String text) {
  final out = <String, String>{};
  for (final raw in text.split('\n')) {
    final line = raw.trim();
    if (line.isEmpty) continue;
    final eq = line.indexOf('=');
    if (eq <= 0) throw FormatException('Write each line as KEY=VALUE: "$line"');
    out[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
  }
  return out;
}
