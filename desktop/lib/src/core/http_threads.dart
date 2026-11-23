part of 'http_core_client.dart';

/// Output-thread endpoints of the local API (docs/api.md, "Output threads"). Uses the
/// request helpers [_HttpChat] declares.
mixin _HttpThreads on _HttpChat {
  String _outputPath(String notebookId, String outputId) =>
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/outputs/${Uri.encodeComponent(outputId)}';

  Future<ChatSession> getThread(String notebookId, String outputId) async =>
      chatSessionFromJson(await _json('GET', '${_outputPath(notebookId, outputId)}/thread'));

  Future<ChatSession> setThreadSettings(
    String notebookId,
    String outputId, {
    String? model,
    String? thinkingEffort,
  }) async {
    final body = {'model': ?model, 'thinkingEffort': ?thinkingEffort};
    return chatSessionFromJson(await _json('PATCH', '${_outputPath(notebookId, outputId)}/thread', body: body));
  }

  Stream<ThreadEvent> sendThread(
    String notebookId,
    String outputId,
    String text, {
    List<ChatAttachment> attachments = const [],
  }) async* {
    checkChatMessage(text, attachments);
    final body = {
      'text': text,
      'attachments': [for (final a in attachments) a.toJson()],
    };
    final resp = await _send('POST', '${_outputPath(notebookId, outputId)}/thread/messages', body: body);
    if (resp.statusCode >= 400) {
      final raw = await HttpCoreClient._readLimited(resp);
      final decoded = raw.isEmpty ? null : jsonDecode(raw);
      final msg = decoded is Map<String, Object?> ? decoded['error'] as String? : null;
      throw CoreException(resp.statusCode, msg ?? 'reply failed');
    }
    await for (final e in parseSse(resp.transform(utf8.decoder))) {
      final Object? data;
      try {
        data = jsonDecode(e.data);
      } on FormatException {
        continue;
      }
      final map = jsonObject(data);
      switch (e.event) {
        case 'delta':
          yield ThreadDelta(map['text'] as String? ?? '');
        case 'suggestion':
          yield ThreadSuggestion(suggestionFromJson(map));
        case 'finished':
          yield ThreadFinished(
            chatMessageFromJson(jsonObject(map['user'])),
            chatMessageFromJson(jsonObject(map['assistant'])),
            suggestions: jsonList(map['suggestions']).map(suggestionFromJson).toList(),
          );
        case 'error':
          throw CoreException((map['status'] as num?)?.toInt() ?? 500, map['error'] as String? ?? 'reply failed');
      }
    }
  }

  Future<List<Suggestion>> listSuggestions(String notebookId, String outputId) async {
    final json = await _json('GET', '${_outputPath(notebookId, outputId)}/suggestions');
    return jsonList(json['suggestions']).map(suggestionFromJson).toList();
  }

  Future<List<Suggestion>> generateSuggestions(String notebookId, String outputId) async {
    final json = await _json('POST', '${_outputPath(notebookId, outputId)}/suggestions/generate');
    return jsonList(json['suggestions']).map(suggestionFromJson).toList();
  }

  Future<({Suggestion suggestion, String? promptName})> applySuggestion(
    String notebookId,
    String outputId,
    String suggestionId,
  ) async {
    final path = '${_outputPath(notebookId, outputId)}/suggestions/${Uri.encodeComponent(suggestionId)}/apply';
    final json = await _json('POST', path);
    return (suggestion: suggestionFromJson(jsonObject(json['suggestion'])), promptName: json['promptName'] as String?);
  }

  Future<Suggestion> dismissSuggestion(String notebookId, String outputId, String suggestionId) async {
    final path = '${_outputPath(notebookId, outputId)}/suggestions/${Uri.encodeComponent(suggestionId)}/dismiss';
    return suggestionFromJson(jsonObject((await _json('POST', path))['suggestion']));
  }

  Future<OutputSummary> newVersion(String notebookId, String outputId, {String? fromMessageId}) async {
    final body = {'fromMessageId': ?fromMessageId};
    return outputFromJson(await _json('POST', '${_outputPath(notebookId, outputId)}/versions', body: body));
  }

  Future<OutputSummary> editOutput(String notebookId, String outputId, Map<String, Object?> data) async =>
      outputFromJson(await _json('POST', '${_outputPath(notebookId, outputId)}/edits', body: {'data': data}));
}
