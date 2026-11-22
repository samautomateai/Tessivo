import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../research_state.dart';
import '../theme.dart';
import 'markdown_view.dart';
import 'research_sources.dart';

/// The conversation column: what a question would send, the answers with
/// their references, and the question box.
class ResearchChatPane extends StatefulWidget {
  const ResearchChatPane({super.key, required this.state});

  final ResearchState state;

  @override
  State<ResearchChatPane> createState() => _ResearchChatPaneState();
}

class _ResearchChatPaneState extends State<ResearchChatPane> {
  final _text = TextEditingController();
  final _scroll = ScrollController();

  @override
  void didUpdateWidget(ResearchChatPane old) {
    super.didUpdateWidget(old);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
    });
  }

  @override
  void dispose() {
    _text.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _text.text;
    if (text.trim().isEmpty || widget.state.streaming) return;
    _text.clear();
    final sent = await widget.state.send(text);
    if (!sent && mounted && _text.text.isEmpty) _text.text = text;
  }

  @override
  Widget build(BuildContext context) {
    final s = widget.state;
    final ctx = s.context;
    final messages = s.chat?.session.messages ?? const <ChatMessage>[];
    return Column(
      key: const Key('research-chat'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Header(state: s),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
          color: ctx.cut.isEmpty ? null : Colors.amber.withValues(alpha: 0.15),
          child: Tooltip(
            message: ctx.cut.isEmpty ? 'What a question sends now' : 'Shortened to fit: ${ctx.cut.join(', ')}',
            child: Text(
              ctx.label,
              key: const Key('research-context-bar'),
              style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
            ),
          ),
        ),
        const Divider(height: 1),
        Expanded(
          child: messages.isEmpty && !s.streaming
              ? const Center(
                  child: Text('Ask about your sources and notes.', style: TextStyle(color: Tokens.textMuted)),
                )
              : ListView(
                  controller: _scroll,
                  padding: const EdgeInsets.all(16),
                  children: [
                    for (final m in messages) _Answer(state: s, message: m),
                    if (s.streaming)
                      _Answer(
                        state: s,
                        message: ChatMessage(
                          id: '',
                          role: 'assistant',
                          content: s.streamedText.isEmpty ? '…' : s.streamedText,
                        ),
                      ),
                    if (s.skipped.isNotEmpty)
                      Text(
                        'Skipped (could not read): ${s.skipped.join(', ')}',
                        style: const TextStyle(color: Tokens.textMuted, fontStyle: FontStyle.italic),
                      ),
                  ],
                ),
        ),
        _composer(s),
      ],
    );
  }

  Widget _composer(ResearchState s) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: TextField(
            key: const Key('research-input'),
            controller: _text,
            minLines: 1,
            maxLines: 6,
            decoration: const InputDecoration(hintText: 'Ask a question', border: OutlineInputBorder()),
          ),
        ),
        const SizedBox(width: 8),
        if (s.streaming)
          OutlinedButton.icon(
            key: const Key('research-stop'),
            onPressed: () => unawaited(s.stop()),
            icon: const Icon(Icons.stop),
            label: const Text('Stop'),
          )
        else
          FilledButton(
            key: const Key('research-send'),
            onPressed: () => unawaited(_send()),
            style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
            child: const Text('Ask'),
          ),
      ],
    ),
  );
}

/// Conversation picker and a new-conversation button.
class _Header extends StatelessWidget {
  const _Header({required this.state});

  final ResearchState state;

  @override
  Widget build(BuildContext context) {
    final current = state.chat?.session.id;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 10, 8, 6),
      child: Row(
        children: [
          Expanded(
            child: state.chats.isEmpty
                ? const Text('Conversation', style: TextStyle(fontWeight: FontWeight.w600))
                : DropdownButton<String>(
                    key: const Key('research-chats'),
                    isExpanded: true,
                    underline: const SizedBox.shrink(),
                    value: state.chats.any((c) => c.id == current) ? current : null,
                    hint: const Text('Conversation'),
                    items: [
                      for (final c in state.chats)
                        DropdownMenuItem(
                          value: c.id,
                          child: Text(c.title, overflow: TextOverflow.ellipsis),
                        ),
                    ],
                    onChanged: (id) {
                      if (id != null) unawaited(state.openChat(id));
                    },
                  ),
          ),
          TextButton.icon(
            key: const Key('research-new-chat'),
            onPressed: state.streaming ? null : () => unawaited(state.newChat()),
            icon: const Icon(Icons.add_comment_outlined, size: 16),
            label: const Text('New'),
          ),
        ],
      ),
    );
  }
}

/// One turn; an answer shows its references and can be saved as a note.
class _Answer extends StatelessWidget {
  const _Answer({required this.state, required this.message});

  final ResearchState state;
  final ChatMessage message;

  Future<void> _open(BuildContext context, Citation c) async {
    if (c.kind == 'note') {
      final note = state.notes.where((n) => n.id == c.id).firstOrNull;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text(c.label),
          content: SizedBox(width: 600, child: SelectableText(note?.text ?? 'This note was deleted.')),
          actions: [TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Close'))],
        ),
      );
      return;
    }
    await showSourceText(context, state, c.id, c.label, path: c.path, summary: c.level == 'summary');
  }

  @override
  Widget build(BuildContext context) {
    final m = message;
    final scheme = Theme.of(context).colorScheme;
    final cites = m.id.isEmpty ? const <Citation>[] : state.chat?.citations[m.id] ?? const <Citation>[];
    return Align(
      alignment: m.isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 760),
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: m.isUser ? scheme.primaryContainer : scheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(10),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (m.isUser) SelectableText(m.content) else MarkdownView(m.content),
            if (cites.isNotEmpty) ...[
              const SizedBox(height: 8),
              const Text('References', style: TextStyle(fontWeight: FontWeight.w600, fontSize: 12)),
              for (final c in cites)
                TextButton(
                  key: Key('cite-${c.n}'),
                  style: TextButton.styleFrom(visualDensity: VisualDensity.compact, alignment: Alignment.centerLeft),
                  onPressed: () => unawaited(_open(context, c)),
                  child: Text('[${c.n}] ${c.reference}', style: const TextStyle(fontSize: 12)),
                ),
            ],
            if (!m.isUser && m.id.isNotEmpty && m.content.isNotEmpty)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton.icon(
                  key: Key('save-note-${m.id}'),
                  onPressed: () => unawaited(state.noteFromMessage(m.id)),
                  icon: const Icon(Icons.note_add_outlined, size: 16),
                  label: const Text('Save to note'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
