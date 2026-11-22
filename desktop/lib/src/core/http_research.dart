part of 'http_core_client.dart';

/// Research notebook endpoints of the local API (docs/api.md, "Research notebooks").
mixin _HttpResearch on _HttpChat {
  String _rPath(String id) => '/v1/research/${Uri.encodeComponent(id)}';

  Future<List<ResearchNotebook>> listResearch() async =>
      jsonList((await _json('GET', '/v1/research'))['notebooks']).map(ResearchNotebook.fromJson).toList();

  Future<ResearchNotebook> createResearch(String name, {String description = ''}) async =>
      ResearchNotebook.fromJson(await _json('POST', '/v1/research', body: {'name': name, 'description': description}));

  Future<ResearchNotebook> getResearch(String id) async => ResearchNotebook.fromJson(await _json('GET', _rPath(id)));

  Future<void> deleteResearch(String id) async {
    await _json('DELETE', _rPath(id));
  }

  Future<ResearchNotebook> addResearchSource(String id, {String? path, String? server, String? uri}) async =>
      ResearchNotebook.fromJson(
        await _json('POST', '${_rPath(id)}/sources', body: {'path': ?path, 'server': ?server, 'uri': ?uri}),
      );

  Future<ResearchNotebook> setSourceLevel(String id, String sourceId, String level) async => ResearchNotebook.fromJson(
    await _json('PATCH', '${_rPath(id)}/sources/${Uri.encodeComponent(sourceId)}', body: {'level': level}),
  );

  Future<ResearchNotebook> removeResearchSource(String id, String sourceId) async =>
      ResearchNotebook.fromJson(await _json('DELETE', '${_rPath(id)}/sources/${Uri.encodeComponent(sourceId)}'));

  Future<void> refreshSummary(String id, String sourceId) async {
    await _json('POST', '${_rPath(id)}/sources/${Uri.encodeComponent(sourceId)}/summary');
  }

  Future<ResearchSourceText> researchSource(String id, String sourceId) async {
    final j = await _json('GET', '${_rPath(id)}/sources/${Uri.encodeComponent(sourceId)}');
    return ResearchSourceText(text: j['text'] as String? ?? '', summary: j['summary'] as String? ?? '');
  }

  Future<ContextTotals> researchContext(String id) async =>
      ContextTotals.fromJson(await _json('GET', '${_rPath(id)}/context'));

  Future<List<ChatSummary>> listResearchChats(String id) async =>
      jsonList((await _json('GET', '${_rPath(id)}/chats'))['sessions']).map(chatSummaryFromJson).toList();

  Future<ChatSession> createResearchChat(String id, {String title = ''}) async =>
      chatSessionFromJson(await _json('POST', '${_rPath(id)}/chats', body: {'title': title}));

  Future<ResearchChat> getResearchChat(String id, String chatId) async {
    final j = await _json('GET', '${_rPath(id)}/chats/${Uri.encodeComponent(chatId)}');
    final raw = j['citations'];
    return ResearchChat(
      session: chatSessionFromJson(jsonObject(j['session'])),
      citations: {
        if (raw is Map<String, Object?>)
          for (final e in raw.entries) e.key: jsonList(e.value).map(Citation.fromJson).toList(),
      },
    );
  }

  Stream<ResearchEvent> sendResearch(String id, String chatId, String text) async* {
    final resp = await _send(
      'POST',
      '${_rPath(id)}/chats/${Uri.encodeComponent(chatId)}/messages',
      body: {'text': text},
    );
    if (resp.statusCode >= 400) {
      final raw = await HttpCoreClient._readLimited(resp);
      final decoded = raw.isEmpty ? null : jsonDecode(raw);
      throw CoreException(
        resp.statusCode,
        (decoded is Map<String, Object?> ? decoded['error'] as String? : null) ?? 'question failed',
      );
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
          yield ResearchDelta(map['text'] as String? ?? '');
        case 'finished':
          yield ResearchFinished(
            chatMessageFromJson(jsonObject(map['user'])),
            chatMessageFromJson(jsonObject(map['assistant'])),
            citations: jsonList(map['citations']).map(Citation.fromJson).toList(),
            skipped: [for (final s in map['skipped'] is List ? map['skipped']! as List : const []) ?s as String?],
          );
        case 'error':
          throw CoreException((map['status'] as num?)?.toInt() ?? 500, map['error'] as String? ?? 'question failed');
      }
    }
  }

  Future<List<ResearchNote>> researchNotes(String id) async =>
      jsonList((await _json('GET', '${_rPath(id)}/notes'))['notes']).map(ResearchNote.fromJson).toList();

  Future<ResearchNote> saveResearchNote(String id, ResearchNote note) async {
    final body = {'title': note.title, 'text': note.text, 'include': note.include};
    final j = note.id.isEmpty
        ? await _json('POST', '${_rPath(id)}/notes', body: body)
        : await _json('PATCH', '${_rPath(id)}/notes/${Uri.encodeComponent(note.id)}', body: body);
    return ResearchNote.fromJson(j);
  }

  Future<void> deleteResearchNote(String id, String noteId) async {
    await _json('DELETE', '${_rPath(id)}/notes/${Uri.encodeComponent(noteId)}');
  }

  Future<ResearchNote> noteFromMessage(String id, String chatId, String messageId) async => ResearchNote.fromJson(
    await _json(
      'POST',
      '${_rPath(id)}/chats/${Uri.encodeComponent(chatId)}/messages/${Uri.encodeComponent(messageId)}/note',
    ),
  );
}
