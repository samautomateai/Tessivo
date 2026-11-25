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
part 'http_mcp.dart';
part 'http_research.dart';
part 'http_sse.dart';
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
class HttpCoreClient with _HttpChat, _HttpThreads, _HttpMcp, _HttpResearch implements CoreClient {
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
  Future<Notebook> updateNotebook(
    String notebookId, {
    String? name,
    bool? archived,
    List<String>? tags,
    bool? pinned,
  }) async => notebookFromJson(
    await _json(
      'PATCH',
      _nb(notebookId),
      body: {'name': ?name, 'archived': ?archived, 'tags': ?tags, 'pinned': ?pinned},
    ),
  );

  @override
  Future<void> deleteNotebook(String notebookId) async {
    await _json('DELETE', _nb(notebookId));
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
  Future<List<SavedPrompt>> listPrompts(String notebookId) async =>
      jsonList((await _json('GET', '${_nb(notebookId)}/prompts'))['prompts']).map(SavedPrompt.fromJson).toList();

  @override
  Future<SavedPrompt> savePrompt(String notebookId, SavedPrompt prompt) async => SavedPrompt.fromJson(
    await _json('PUT', '${_nb(notebookId)}/prompts/${Uri.encodeComponent(prompt.name)}', body: prompt.toBody()),
  );

  @override
  Future<void> deletePrompt(String notebookId, String name) async {
    await _json('DELETE', '${_nb(notebookId)}/prompts/${Uri.encodeComponent(name)}');
  }

  @override
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
    String? promptName,
    String? model,
  }) async* {
    final resp = await _send(
      'POST',
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/runs',
      body: {
        'prompt': prompt,
        'vars': vars,
        'promptName': ?promptName,
        if (model != null && model.isNotEmpty) 'overrides': {'model': model},
      },
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
  Future<Notebook> addUrlContext(String notebookId, String url, {String label = ''}) async =>
      notebookFromJson(await _json('POST', '${_nb(notebookId)}/context', body: {'url': url, 'label': label}));

  @override
  Future<Notebook> addTextContext(String notebookId, String text, {String label = ''}) async =>
      notebookFromJson(await _json('POST', '${_nb(notebookId)}/context', body: {'text': text, 'label': label}));

  @override
  Future<Notebook> addGitDiffContext(String notebookId, String repo, String base, {String label = ''}) async =>
      notebookFromJson(
        await _json(
          'POST',
          '${_nb(notebookId)}/context',
          body: {
            'git': {'repo': repo, 'base': base},
            'label': label,
          },
        ),
      );

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
  Future<Uint8List> exportOutput(String notebookId, String outputId, String format) =>
      _bytes('${_nb(notebookId)}/outputs/${Uri.encodeComponent(outputId)}/export', query: {'format': format});

  @override
  Future<Uint8List> diagnostics() => _bytes('/v1/diagnostics');

  @override
  Future<RunInfo> outputRun(String notebookId, String outputId) async =>
      RunInfo.fromJson(await _json('GET', '${_nb(notebookId)}/outputs/${Uri.encodeComponent(outputId)}/run'));

  @override
  Future<OutputDiffInfo> diffOutput(String notebookId, String outputId, {String? against}) async =>
      OutputDiffInfo.fromJson(
        await _json(
          'GET',
          '${_nb(notebookId)}/outputs/${Uri.encodeComponent(outputId)}/diff',
          query: {'against': ?against},
        ),
      );

  @override
  Future<List<SearchHit>> search(String query) async =>
      jsonList((await _json('GET', '/v1/search', query: {'q': query}))['results']).map(SearchHit.fromJson).toList();

  /// A file the core sends (an export, diagnostics), bounded like JSON replies.
  Future<Uint8List> _bytes(String path, {Map<String, String>? query}) async {
    final resp = await _send('GET', path, query: query);
    final builder = BytesBuilder(copy: false);
    await for (final chunk in resp) {
      builder.add(chunk);
      if (builder.length > maxResponseBytes) throw const CoreException(502, 'file too large');
    }
    final bytes = builder.takeBytes();
    if (resp.statusCode >= 400) {
      final decoded = bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes, allowMalformed: true));
      throw CoreException(
        resp.statusCode,
        (decoded is Map<String, Object?> ? decoded['error'] as String? : null) ?? 'download failed',
      );
    }
    return bytes;
  }

  @override
  Future<SettingsInfo> getSettings() async => settingsFromJson(await _json('GET', '/v1/settings'));

  @override
  Future<List<LibraryPrompt>> promptLibrary({String? workType}) async {
    final q = workType == null ? '' : '?workType=${Uri.encodeQueryComponent(workType)}';
    return libraryFromJson(await _json('GET', '/v1/prompt-library$q'));
  }

  @override
  Future<UsageInfo> usage() async => UsageInfo.fromJson(await _json('GET', '/v1/usage'));

  @override
  Future<List<ModelCheck>> testModel(String model) async =>
      modelChecksFromJson(await _json('POST', '/v1/models/test', body: {'model': model}));

  @override
  Future<SettingsInfo> updateSettings(Map<String, Object?> body) async =>
      settingsFromJson(await _json('PUT', '/v1/settings', body: body));

  @override
  Future<bool> secretSet(String ref) async =>
      (await _json('GET', '/v1/secrets/${Uri.encodeComponent(ref)}'))['set'] == true;

  @override
  Future<void> setSecret(String ref, String value) async {
    await _json('PUT', '/v1/secrets/${Uri.encodeComponent(ref)}', body: {'value': value});
  }

  @override
  Future<void> deleteSecret(String ref) async {
    await _json('DELETE', '/v1/secrets/${Uri.encodeComponent(ref)}');
  }

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
