import 'dart:async';

import 'chat_models.dart';
import 'errors.dart';
import 'research_models.dart';

/// In-memory research notebooks for the demo core. Summaries are ready as
/// soon as a source is added; answers cite the first source in context.
mixin DemoResearch {
  /// Delay between streamed words.
  Duration get chatTick;

  final List<ResearchNotebook> _research = [
    const ResearchNotebook(
      id: 'board-papers',
      name: 'Board papers',
      description: 'Q3 board meeting',
      notes: 1,
      sources: [
        ResearchSource(id: 'rs1', label: 'minutes.md', kind: 'file', level: 'summary', summary: 'ready', tokens: 3200),
        ResearchSource(
          id: 'rs2',
          label: 'payments-service',
          kind: 'folder',
          level: 'full',
          summary: 'ready',
          tokens: 38000,
        ),
      ],
    ),
  ];
  final Map<String, List<ResearchNote>> _researchNotes = {
    'board-papers': [
      const ResearchNote(id: 'rn1', title: 'Audit follow-up', text: 'Ask finance about the Q2 audit.', origin: 'human'),
    ],
  };
  final Map<String, ChatSession> _researchChats = {};
  final Map<String, Map<String, List<Citation>>> _researchCites = {};
  int _researchSeq = 0;

  ResearchNotebook _rnb(String id) => _research.firstWhere(
    (n) => n.id == id,
    orElse: () => throw CoreException(404, 'research notebook $id not found'),
  );

  void _rput(ResearchNotebook nb) {
    final i = _research.indexWhere((n) => n.id == nb.id);
    _research[i] = nb;
  }

  ResearchNotebook _withSources(ResearchNotebook nb, List<ResearchSource> sources) => ResearchNotebook(
    id: nb.id,
    name: nb.name,
    description: nb.description,
    sources: sources,
    notes: _researchNotes[nb.id]?.length ?? 0,
  );

  Future<List<ResearchNotebook>> listResearch() async => List.unmodifiable(_research);

  Future<ResearchNotebook> createResearch(String name, {String description = ''}) async {
    if (name.trim().isEmpty) throw const CoreException(400, 'name is required');
    final id = '${name.trim().toLowerCase().replaceAll(RegExp(r'[^a-z0-9]+'), '-')}-${++_researchSeq}';
    final nb = ResearchNotebook(id: id, name: name.trim(), description: description);
    _research.insert(0, nb);
    return nb;
  }

  Future<ResearchNotebook> getResearch(String id) async => _rnb(id);

  Future<void> deleteResearch(String id) async => _research.removeWhere((n) => n.id == id);

  Future<ResearchNotebook> addResearchSource(String id, {String? path, String? server, String? uri}) async {
    final nb = _rnb(id);
    final label = path != null ? path.split(RegExp(r'[/\\]')).where((p) => p.isNotEmpty).last : (uri ?? '');
    final source = ResearchSource(
      id: 'src${++_researchSeq}',
      label: label,
      kind: path != null ? (label.contains('.') ? 'file' : 'folder') : 'mcp-resource',
      level: 'summary',
      summary: 'ready',
      tokens: 4000,
    );
    final updated = _withSources(nb, [...nb.sources, source]);
    _rput(updated);
    return updated;
  }

  Future<ResearchNotebook> setSourceLevel(String id, String sourceId, String level) async {
    final nb = _rnb(id);
    final updated = _withSources(nb, [for (final s in nb.sources) s.id == sourceId ? s.copyWith(level: level) : s]);
    _rput(updated);
    return updated;
  }

  Future<ResearchNotebook> removeResearchSource(String id, String sourceId) async {
    final nb = _rnb(id);
    final updated = _withSources(nb, nb.sources.where((s) => s.id != sourceId).toList());
    _rput(updated);
    return updated;
  }

  Future<void> refreshSummary(String id, String sourceId) async => _rnb(id);

  Future<ResearchSourceText> researchSource(String id, String sourceId) async {
    final s = _rnb(id).sources.firstWhere((s) => s.id == sourceId);
    return ResearchSourceText(
      text: 'Sample text of ${s.label}.\n\nIn this demo the core is not running, so this is not the real file.',
      summary: '${s.label}: a sample summary of its main points.',
    );
  }

  Future<ContextTotals> researchContext(String id) async {
    final nb = _rnb(id);
    final on = nb.sources.where((s) => s.level != 'off').toList();
    final notes = (_researchNotes[id] ?? const []).where((n) => n.include).length;
    final tokens = on.fold(0, (sum, s) => sum + (s.level == 'summary' ? 600 : s.tokens)) + notes * 200;
    return ContextTotals(
      sources: on.length,
      summaries: on.where((s) => s.level == 'summary').length,
      notes: notes,
      tokens: tokens,
      cut: tokens > 38000 ? [on.first.label] : const [],
    );
  }

  Future<List<ChatSummary>> listResearchChats(String id) async => [
    for (final c in _researchChats.entries)
      if (c.key.startsWith('$id/'))
        ChatSummary(id: c.value.id, title: c.value.title, messageCount: c.value.messages.length),
  ];

  Future<ChatSession> createResearchChat(String id, {String title = ''}) async {
    _rnb(id);
    final chat = ChatSession(id: 'rc${++_researchSeq}', title: title.isEmpty ? 'New conversation' : title);
    _researchChats['$id/${chat.id}'] = chat;
    return chat;
  }

  Future<ResearchChat> getResearchChat(String id, String chatId) async {
    final chat = _researchChats['$id/$chatId'] ?? (throw CoreException(404, 'conversation $chatId not found'));
    return ResearchChat(session: chat, citations: Map.unmodifiable(_researchCites['$id/$chatId'] ?? const {}));
  }

  Stream<ResearchEvent> sendResearch(String id, String chatId, String text) async* {
    final nb = _rnb(id);
    final chat = (await getResearchChat(id, chatId)).session;
    final first = nb.sources.where((s) => s.level != 'off').firstOrNull;
    final answer = first == null
        ? 'There are no sources in context. Switch one on to ask about it.'
        : 'According to ${first.label}, the decision was to ship in March [1].';
    for (final w in answer.split(' ')) {
      await Future<void>.delayed(chatTick);
      yield ResearchDelta('$w ');
    }
    final user = ChatMessage(id: 'ru${++_researchSeq}', role: 'user', content: text);
    final reply = ChatMessage(id: 'ra$_researchSeq', role: 'assistant', content: answer);
    final cites = [
      if (first != null) Citation(n: 1, kind: 'source', id: first.id, label: first.label, level: first.level),
    ];
    _researchChats['$id/$chatId'] = chat.copyWith(messages: [...chat.messages, user, reply]);
    (_researchCites['$id/$chatId'] ??= {})[reply.id] = cites;
    yield ResearchFinished(user, reply, citations: cites);
  }

  Future<List<ResearchNote>> researchNotes(String id) async => List.unmodifiable(_researchNotes[id] ?? const []);

  Future<ResearchNote> saveResearchNote(String id, ResearchNote note) async {
    final list = _researchNotes[id] ??= [];
    final saved = ResearchNote(
      id: note.id.isEmpty ? 'note${++_researchSeq}' : note.id,
      title: note.title,
      text: note.text,
      origin: note.origin,
      include: note.include,
    );
    final i = list.indexWhere((n) => n.id == saved.id);
    if (i < 0) {
      list.add(saved);
    } else {
      list[i] = saved;
    }
    return saved;
  }

  Future<void> deleteResearchNote(String id, String noteId) async =>
      _researchNotes[id]?.removeWhere((n) => n.id == noteId);

  Future<ResearchNote> noteFromMessage(String id, String chatId, String messageId) async {
    final chat = (await getResearchChat(id, chatId)).session;
    final i = chat.messages.indexWhere((m) => m.id == messageId);
    if (i < 0) throw CoreException(404, 'message $messageId not found');
    final question = i > 0 ? chat.messages[i - 1].content : 'Saved answer';
    return saveResearchNote(id, ResearchNote(id: '', title: question, text: chat.messages[i].content, origin: 'ai'));
  }
}
