import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'core_client.dart';
import 'models.dart';

/// Connection details the core writes to `run/endpoint.json` (0600).
class Endpoint {
  const Endpoint({required this.network, required this.address, required this.token});

  factory Endpoint.fromJson(Map<String, Object?> json) {
    final network = json['network'];
    final address = json['address'];
    final token = json['token'];
    if (network is! String || address is! String || token is! String || token.isEmpty) {
      throw const FormatException('endpoint file is missing network, address or token');
    }
    if (network != 'unix' && network != 'tcp') {
      throw FormatException('unknown network $network');
    }
    return Endpoint(network: network, address: address, token: token);
  }

  final String network;
  final String address;
  final String token;

  /// Reads the endpoint file; returns null when the core is not running.
  static Future<Endpoint?> discover({String? path, Map<String, String>? environment}) async {
    final file = File(path ?? defaultEndpointPath(environment ?? Platform.environment));
    if (!await file.exists()) return null;
    final raw = await file.readAsString();
    if (raw.length > 64 * 1024) throw const FormatException('endpoint file is too large');
    final decoded = jsonDecode(raw);
    if (decoded is! Map<String, Object?>) throw const FormatException('endpoint file is not an object');
    return Endpoint.fromJson(decoded);
  }
}

/// Mirrors Go's os.UserConfigDir()/tessivo/run/endpoint.json.
String defaultEndpointPath(Map<String, String> env, {String? os}) {
  final platform = os ?? Platform.operatingSystem;
  final String base;
  switch (platform) {
    case 'windows':
      base = env['APPDATA'] ?? '';
    case 'macos':
      base = '${env['HOME'] ?? ''}/Library/Application Support';
    default:
      final xdg = env['XDG_CONFIG_HOME'];
      base = (xdg != null && xdg.isNotEmpty) ? xdg : '${env['HOME'] ?? ''}/.config';
  }
  final sep = platform == 'windows' ? r'\' : '/';
  return [base, 'tessivo', 'run', 'endpoint.json'].join(sep);
}

/// An error returned by the core.
class CoreException implements Exception {
  const CoreException(this.status, this.message);
  final int status;
  final String message;

  @override
  String toString() => 'CoreException($status): $message';
}

/// Talks to the Go core's local API (docs/api.md) over a Unix socket or a
/// loopback port, with the session token on every request.
class HttpCoreClient implements CoreClient {
  HttpCoreClient(this.endpoint, {HttpClient? client}) : _client = client ?? HttpClient() {
    _client
      ..findProxy = ((_) => 'DIRECT')
      ..connectionTimeout = const Duration(seconds: 5)
      ..idleTimeout = const Duration(seconds: 30);
    if (endpoint.network == 'unix') {
      final address = InternetAddress(endpoint.address, type: InternetAddressType.unix);
      _client.connectionFactory = (uri, proxyHost, proxyPort) => Socket.startConnect(address, 0);
    }
  }

  /// Largest JSON response accepted (bytes).
  static const maxResponseBytes = 16 * 1024 * 1024;

  final Endpoint endpoint;
  final HttpClient _client;

  Uri _uri(String path, [Map<String, String>? query]) {
    final host = endpoint.network == 'unix' ? 'tessivo' : endpoint.address;
    return Uri.parse('http://$host$path').replace(queryParameters: query);
  }

  Future<HttpClientResponse> _send(String method, String path, {Object? body, Map<String, String>? query}) async {
    final req = await _client.openUrl(method, _uri(path, query));
    req.headers
      ..set(HttpHeaders.authorizationHeader, 'Bearer ${endpoint.token}')
      ..set(HttpHeaders.acceptHeader, 'application/json, text/event-stream');
    if (body != null) {
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode(body));
    }
    return req.close();
  }

  Future<Map<String, Object?>> _json(String method, String path, {Object? body, Map<String, String>? query}) async {
    final resp = await _send(method, path, body: body, query: query);
    final text = await _readLimited(resp);
    final decoded = text.isEmpty ? <String, Object?>{} : jsonDecode(text);
    final map = decoded is Map<String, Object?> ? decoded : <String, Object?>{};
    if (resp.statusCode >= 400) {
      throw CoreException(resp.statusCode, (map['error'] as String?) ?? 'request failed');
    }
    return map;
  }

  static Future<String> _readLimited(HttpClientResponse resp) async {
    final builder = BytesBuilder(copy: false);
    await for (final chunk in resp) {
      builder.add(chunk);
      if (builder.length > maxResponseBytes) {
        throw const CoreException(502, 'response too large');
      }
    }
    return utf8.decode(builder.takeBytes());
  }

  /// True when the core answers its health check.
  Future<bool> health() async {
    final json = await _json('GET', '/v1/health');
    return json['ok'] == true;
  }

  @override
  Future<List<WorkType>> listWorkTypes() async => builtInWorkTypes;

  @override
  Future<List<Notebook>> listNotebooks() async {
    final json = await _json('GET', '/v1/notebooks');
    return _list(json['notebooks']).map(notebookFromJson).toList();
  }

  @override
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType}) async {
    return notebookFromJson(await _json('POST', '/v1/notebooks', body: {'name': name, 'workType': workType.name}));
  }

  @override
  Future<Notebook> setContextEnabled({
    required String notebookId,
    required String contextId,
    required bool enabled,
  }) async {
    final path = '/v1/notebooks/${Uri.encodeComponent(notebookId)}/context/${Uri.encodeComponent(contextId)}';
    return notebookFromJson(await _json('PATCH', path, body: {'enabled': enabled}));
  }

  @override
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType}) async {
    final json = await _json('GET', '/v1/outputs', query: workType == null ? null : {'workType': workType.name});
    return _list(json['outputs']).map(outputFromJson).toList();
  }

  @override
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
  }) async* {
    final resp = await _send(
      'POST',
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/runs',
      body: {'prompt': prompt, 'vars': vars},
    );
    if (resp.statusCode >= 400) {
      final text = await _readLimited(resp);
      final decoded = text.isEmpty ? null : jsonDecode(text);
      final msg = decoded is Map<String, Object?> ? decoded['error'] as String? : null;
      throw CoreException(resp.statusCode, msg ?? 'run failed');
    }
    await for (final e in parseSse(resp.transform(utf8.decoder))) {
      final data = jsonDecode(e.data);
      final map = data is Map<String, Object?> ? data : <String, Object?>{};
      switch (e.event) {
        case 'status':
          yield RunStatus((map['text'] as String?) ?? '');
        case 'delta':
          yield RunDelta((map['text'] as String?) ?? '');
        case 'finished':
          yield RunFinished(outcomeFromJson(map));
        case 'error':
          throw CoreException((map['status'] as num?)?.toInt() ?? 500, (map['error'] as String?) ?? 'run failed');
      }
    }
  }

  /// Releases connections.
  void close() => _client.close(force: true);
}

List<Map<String, Object?>> _list(Object? v) =>
    v is List ? v.whereType<Map<String, Object?>>().toList() : const <Map<String, Object?>>[];

WorkTypeId _workType(Object? v) =>
    WorkTypeId.values.firstWhere((w) => w.name == v, orElse: () => WorkTypeId.summarize);

Notebook notebookFromJson(Map<String, Object?> j) => Notebook(
  id: j['id'] as String? ?? '',
  name: j['name'] as String? ?? '',
  workType: _workType(j['workType']),
  context: _list(j['context']).map(contextFromJson).toList(),
);

ContextItem contextFromJson(Map<String, Object?> j) {
  final label = j['label'] as String? ?? '';
  final dot = label.lastIndexOf('.');
  final kind = (j['inPlace'] as bool? ?? false)
      ? 'Folder'
      : (dot > 0 ? label.substring(dot + 1).toUpperCase() : 'File');
  return ContextItem(
    id: j['id'] as String? ?? '',
    label: label,
    kind: kind,
    enabled: j['enabled'] as bool? ?? false,
    estimatedTokens: (j['estimatedTokens'] as num?)?.toInt() ?? 0,
  );
}

OutputSummary outputFromJson(Map<String, Object?> j) => OutputSummary(
  id: j['id'] as String? ?? '',
  title: j['title'] as String? ?? '',
  workType: _workType(j['workType']),
  notebookName: j['notebookName'] as String? ?? '',
  trigger: 'Manual',
  createdAt: j['createdAt'] as String? ?? '',
  versions: 1,
);

RunOutcome outcomeFromJson(Map<String, Object?> j) {
  final attempts = _list(j['attempts']);
  final fell = attempts.where((a) => (a['trigger'] as String? ?? '').isNotEmpty).toList();
  return RunOutcome(
    valid: j['valid'] as bool? ?? false,
    modelUsed: j['modelUsed'] as String? ?? '',
    totalTokens: (j['totalTokens'] as num?)?.toInt() ?? 0,
    markdown: j['markdown'] as String? ?? '',
    fallbackNote: fell.isEmpty ? null : 'fell back after ${fell.map((a) => a['trigger']).join(', ')}',
  );
}

/// One server-sent event.
class SseEvent {
  const SseEvent(this.event, this.data);
  final String event;
  final String data;
}

/// Parses a text/event-stream. Lines longer than [maxLine] are rejected.
Stream<SseEvent> parseSse(Stream<String> chunks, {int maxLine = 4 * 1024 * 1024}) async* {
  var event = 'message';
  final data = StringBuffer();
  await for (final line in chunks.transform(const LineSplitter())) {
    if (line.length > maxLine) throw const CoreException(502, 'event too large');
    if (line.isEmpty) {
      if (data.isNotEmpty) yield SseEvent(event, data.toString());
      event = 'message';
      data.clear();
    } else if (line.startsWith('event:')) {
      event = line.substring(6).trim();
    } else if (line.startsWith('data:')) {
      if (data.isNotEmpty) data.write('\n');
      data.write(line.substring(5).trimLeft());
    }
  }
  if (data.isNotEmpty) yield SseEvent(event, data.toString());
}
