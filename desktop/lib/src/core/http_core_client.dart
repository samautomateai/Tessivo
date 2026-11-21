import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'core_client.dart';
import 'json_mapping.dart';
import 'models.dart';

export 'errors.dart';
export 'json_mapping.dart';

part 'http_chat.dart';
part 'http_threads.dart';

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
    const maxBytes = 64 * 1024;
    final String raw;
    try {
      // Check the size before reading: the file is outside our control.
      if (await file.length() > maxBytes) throw const FormatException('endpoint file is too large');
      raw = await file.readAsString();
    } on PathNotFoundException {
      return null; // the core is not running
    }
    if (raw.length > maxBytes) throw const FormatException('endpoint file is too large');
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

/// Talks to the Go core's local API (docs/api.md) over a Unix socket or a
/// loopback port, with the session token on every request.
class HttpCoreClient with _HttpChat, _HttpThreads implements CoreClient {
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

  @override
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

  @override
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
    return jsonList(json['notebooks']).map(notebookFromJson).toList();
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
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType, String? notebookId}) async {
    final query = {'workType': ?workType?.name, 'notebook': ?notebookId};
    final json = await _json('GET', '/v1/outputs', query: query.isEmpty ? null : query);
    return jsonList(json['outputs']).map(outputFromJson).toList();
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
        case 'tool_call':
          yield RunToolCall(tool: (map['tool'] as String?) ?? '', args: (map['args'] as String?) ?? '');
        case 'tool_result':
          yield RunToolResult(tool: (map['tool'] as String?) ?? '', result: (map['result'] as String?) ?? '');
        case 'paused':
          yield RunPaused(
            runId: (map['runId'] as String?) ?? '',
            approvals: jsonList(map['approvals']).map(approvalFromJson).toList(),
          );
        case 'error':
          throw CoreException((map['status'] as num?)?.toInt() ?? 500, (map['error'] as String?) ?? 'run failed');
      }
    }
  }

  String _nb(String id) => '/v1/notebooks/${Uri.encodeComponent(id)}';

  @override
  Future<Notebook> getNotebook(String notebookId) async => notebookFromJson(await _json('GET', _nb(notebookId)));

  @override
  Future<Notebook> addContext(String notebookId, String path) async {
    checkContextPath(path);
    return notebookFromJson(await _json('POST', '${_nb(notebookId)}/context', body: {'path': path}));
  }

  @override
  Future<Notebook> setMemory(String notebookId, List<String> memory) async {
    checkMemory(memory);
    return notebookFromJson(await _json('PUT', '${_nb(notebookId)}/memory', body: {'memory': memory}));
  }

  @override
  Future<OutputDetail> getOutput(String notebookId, String outputId) async {
    final path = '${_nb(notebookId)}/outputs/${Uri.encodeComponent(outputId)}';
    return outputDetailFromJson(await _json('GET', path));
  }

  @override
  Future<SettingsInfo> getSettings() async => settingsFromJson(await _json('GET', '/v1/settings'));

  @override
  Future<Map<String, List<String>>> listModels({String? provider}) async {
    final query = provider == null ? null : {'provider': provider};
    return modelsFromJson(await _json('GET', '/v1/models', query: query));
  }

  String _schedulesPath(String notebookId) => '/v1/notebooks/${Uri.encodeComponent(notebookId)}/schedules';

  @override
  Future<List<ScheduleInfo>> listSchedules(String notebookId) async {
    final json = await _json('GET', _schedulesPath(notebookId));
    return jsonList(json['schedules']).map(scheduleFromJson).toList();
  }

  @override
  Future<ScheduleInfo> addSchedule(
    String notebookId, {
    required String cron,
    String prompt = '',
    String missed = 'skip',
  }) async {
    final body = {'cron': cron, 'prompt': prompt, 'missed': missed};
    return scheduleFromJson(await _json('POST', _schedulesPath(notebookId), body: body));
  }

  @override
  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled}) async {
    final path = '${_schedulesPath(notebookId)}/${Uri.encodeComponent(scheduleId)}';
    return scheduleFromJson(await _json('PATCH', path, body: {'enabled': enabled}));
  }

  @override
  Future<void> deleteSchedule(String notebookId, String scheduleId) async {
    await _json('DELETE', '${_schedulesPath(notebookId)}/${Uri.encodeComponent(scheduleId)}');
  }

  String _run(String id) => '/v1/runs/${Uri.encodeComponent(id)}';

  @override
  Future<List<PendingApproval>> pendingApprovals() async {
    final json = await _json('GET', '/v1/approvals');
    return jsonList(json['approvals']).map(approvalFromJson).toList();
  }

  @override
  Future<void> answerApproval(String runId, String approvalId, {required bool allow, String? remember}) async {
    final body = {'decision': allow ? 'allow' : 'deny', 'remember': ?remember};
    await _json('POST', '${_run(runId)}/approvals/${Uri.encodeComponent(approvalId)}', body: body);
  }

  @override
  Future<void> cancelRun(String runId) async {
    await _json('POST', '${_run(runId)}/cancel');
  }

  @override
  Future<void> resumeRun(String runId) async {
    await _json('POST', '${_run(runId)}/resume');
  }

  @override
  Stream<CoreEvent> events() async* {
    final resp = await _send('GET', '/v1/events');
    if (resp.statusCode >= 400) {
      await resp.drain<void>();
      throw CoreException(resp.statusCode, 'events unavailable');
    }
    await for (final e in parseSse(resp.transform(utf8.decoder), maxLine: 64 * 1024)) {
      if (e.event != 'event') continue;
      final Object? data;
      try {
        data = jsonDecode(e.data);
      } on FormatException {
        continue; // hostile or garbled frame: drop it, keep the stream
      }
      if (data is Map<String, Object?>) yield eventFromJson(data);
    }
  }

  @override
  Future<List<TemplateInfo>> listTemplates({WorkTypeId? workType}) async {
    final json = await _json('GET', '/v1/templates', query: workType == null ? null : {'workType': workType.name});
    return jsonList(json['templates']).map(templateFromJson).toList();
  }

  @override
  Future<Notebook> createFromTemplate(String templateId, {String? name}) async {
    final body = {'templateId': templateId, if (name != null && name.isNotEmpty) 'name': name};
    return notebookFromJson(await _json('POST', '/v1/notebooks', body: body));
  }

  @override
  Future<TemplateInfo> saveAsTemplate(String notebookId, {required String name, String description = ''}) async {
    final body = {'fromNotebook': notebookId, 'name': name, 'description': description};
    return templateFromJson(await _json('POST', '/v1/templates', body: body));
  }

  @override
  Future<void> deleteTemplate(String templateId) async {
    await _json('DELETE', '/v1/templates/${Uri.encodeComponent(templateId)}');
  }

  /// Releases connections.
  void close() => _client.close(force: true);
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
