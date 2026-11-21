part of 'http_core_client.dart';

/// Chat endpoints of the local API (docs/api.md, "Chat").
mixin _HttpChat {
  Future<HttpClientResponse> _send(String method, String path, {Object? body, Map<String, String>? query});
  Future<Map<String, Object?>> _json(String method, String path, {Object? body, Map<String, String>? query});

  String _chatPath(String id) => '/v1/chats/${Uri.encodeComponent(id)}';

  Future<List<ChatSummary>> listChats({String query = '', bool archived = false}) async {
    final q = {if (query.isNotEmpty) 'q': query, if (archived) 'archived': 'true'};
    final json = await _json('GET', '/v1/chats', query: q.isEmpty ? null : q);
    return jsonList(json['sessions']).map(chatSummaryFromJson).toList();
  }

  Future<ChatSession> createChat({String title = '', String model = '', String thinkingEffort = ''}) async {
    final body = {'title': title, 'model': model, 'thinkingEffort': thinkingEffort};
    return chatSessionFromJson(await _json('POST', '/v1/chats', body: body));
  }

  Future<ChatSession> getChat(String chatId) async => chatSessionFromJson(await _json('GET', _chatPath(chatId)));

  Future<ChatSession> updateChat(
    String chatId, {
    String? title,
    bool? pinned,
    bool? archived,
    String? model,
    String? thinkingEffort,
  }) async {
    final body = {
      'title': ?title,
      'pinned': ?pinned,
      'archived': ?archived,
      'model': ?model,
      'thinkingEffort': ?thinkingEffort,
    };
    return chatSessionFromJson(await _json('PATCH', _chatPath(chatId), body: body));
  }

  Future<void> deleteChat(String chatId) async {
    await _json('DELETE', _chatPath(chatId));
  }

  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments = const []}) async* {
    checkChatMessage(text, attachments);
    final body = {'text': text, 'attachments': [for (final a in attachments) a.toJson()]};
    final resp = await _send('POST', '${_chatPath(chatId)}/messages', body: body);
    if (resp.statusCode >= 400) {
      final raw = await HttpCoreClient._readLimited(resp);
      final decoded = raw.isEmpty ? null : jsonDecode(raw);
      final msg = decoded is Map<String, Object?> ? decoded['error'] as String? : null;
      throw CoreException(resp.statusCode, msg ?? 'chat failed');
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
          yield ChatDelta(map['text'] as String? ?? '');
        case 'finished':
          final user = chatMessageFromJson(jsonObject(map['user']));
          yield ChatFinished(user, chatMessageFromJson(jsonObject(map['assistant'])));
        case 'error':
          throw CoreException((map['status'] as num?)?.toInt() ?? 500, map['error'] as String? ?? 'chat failed');
      }
    }
  }

  Future<List<String>> chatMemory() async {
    final json = await _json('GET', '/v1/chat-memory');
    return jsonStrings(json['memory']);
  }

  Future<List<String>> setChatMemory(List<String> memory) async {
    checkMemory(memory);
    final json = await _json('PUT', '/v1/chat-memory', body: {'memory': memory});
    return jsonStrings(json['memory']);
  }

  Future<RepeatablePlan> planRepeatable(String chatId) async =>
      repeatablePlanFromJson(await _json('GET', '${_chatPath(chatId)}/repeatable'));

  Future<Notebook> makeRepeatable(String chatId, {String? name, WorkTypeId? workType, String? templateId}) async {
    final body = {'name': ?name, 'workType': ?workType?.name, 'templateId': ?templateId};
    return notebookFromJson(await _json('POST', '${_chatPath(chatId)}/notebook', body: body));
  }

  Future<ChatOutput> saveReply(String chatId, String messageId, WorkTypeId workType) async {
    final path = '${_chatPath(chatId)}/messages/${Uri.encodeComponent(messageId)}/output';
    return chatOutputFromJson(await _json('POST', path, body: {'workType': workType.name}));
  }
}
