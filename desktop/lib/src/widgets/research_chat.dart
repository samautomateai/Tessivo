import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/fields.dart';
import '../design/components/modal.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../research_state.dart';
import 'markdown_view.dart';
import 'research_sources.dart';

/// The Ask tab: what a question would send, the answers with their
/// references, and the question box.
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
    final c = TvColors.of(context);
    final s = widget.state;
    final ctx = s.context;
    final messages = s.chat?.session.messages ?? const <ChatMessage>[];
    return Column(
      key: const Key('research-chat'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Header(state: s),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: TvSpace.s24, vertical: TvSpace.s4),
          decoration: BoxDecoration(
            color: ctx.cut.isEmpty ? c.layer01 : c.warning.withValues(alpha: 0.15),
            border: Border(bottom: BorderSide(color: c.borderSubtle)),
          ),
          child: Tooltip(
            message: ctx.cut.isEmpty ? 'What a question sends now' : 'Shortened to fit: ${ctx.cut.join(', ')}',
            child: Text(
              ctx.label,
              key: const Key('research-context-bar'),
              style: TvType.helper.copyWith(color: c.textSecondary),
            ),
          ),
        ),
        Expanded(
          child: messages.isEmpty && !s.streaming
              ? Center(
                  child: Text('Ask about your sources and notes.', style: TvType.body.copyWith(color: c.textSecondary)),
                )
              : ListView(
                  controller: _scroll,
                  padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s16),
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
                        style: TvType.helper.copyWith(color: c.textHelper),
                      ),
                  ],
                ),
        ),
        _composer(c, s),
      ],
    );
  }

  Widget _composer(TvColors c, ResearchState s) => Padding(
    padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s8, TvSpace.s24, TvSpace.s16),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: TextField(
            key: const Key('research-input'),
            controller: _text,
            minLines: 1,
            maxLines: 6,
            style: TvType.body.copyWith(color: c.textPrimary),
            cursorColor: c.interactive,
            decoration: tvFieldDecoration(c, hint: 'Ask a question'),
          ),
        ),
        const SizedBox(width: TvSpace.s8),
        if (s.streaming)
          TvButton('Stop', key: const Key('research-stop'), icon: TvIcon.close, onPressed: () => unawaited(s.stop()))
        else
          TvButton(
            'Ask',
            key: const Key('research-send'),
            icon: TvIcon.send,
            kind: TvButtonKind.primary,
            onPressed: () => unawaited(_send()),
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
    final c = TvColors.of(context);
    final current = state.chat?.session.id;
    return Padding(
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s8, TvSpace.s16, TvSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: state.chats.isEmpty
                ? Text('Conversation', style: TvType.heading01.copyWith(color: c.textPrimary))
                : DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      key: const Key('research-chats'),
                      isExpanded: true,
                      isDense: true,
                      dropdownColor: c.layer02,
                      style: TvType.body.copyWith(color: c.textPrimary),
                      value: state.chats.any((x) => x.id == current) ? current : null,
                      hint: Text('Conversation', style: TvType.body.copyWith(color: c.textHelper)),
                      items: [
                        for (final x in state.chats)
                          DropdownMenuItem(
                            value: x.id,
                            child: Text(x.title, overflow: TextOverflow.ellipsis),
                          ),
                      ],
                      onChanged: (id) {
                        if (id != null) unawaited(state.openChat(id));
                      },
                    ),
                  ),
          ),
          const SizedBox(width: TvSpace.s8),
          TvButton(
            'New conversation',
            key: const Key('research-new-chat'),
            icon: TvIcon.add,
            kind: TvButtonKind.ghost,
            dense: true,
            onPressed: state.streaming ? null : () => unawaited(state.newChat()),
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
      await showTvModal<void>(
        context,
        title: c.label,
        width: 600,
        body: SelectableText(note?.text ?? 'This note was deleted.'),
        actions: const [TvModalAction('Close', value: null)],
      );
      return;
    }
    await showSourceText(context, state, c.id, c.label, path: c.path, summary: c.level == 'summary');
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final m = message;
    final cites = m.id.isEmpty ? const <Citation>[] : state.chat?.citations[m.id] ?? const <Citation>[];
    return Align(
      alignment: Alignment.topLeft,
      child: Container(
        constraints: const BoxConstraints(maxWidth: TvSize.readingWidth),
        margin: const EdgeInsets.only(bottom: TvSpace.s12),
        padding: m.isUser ? const EdgeInsets.all(TvSpace.s12) : const EdgeInsets.symmetric(vertical: TvSpace.s4),
        decoration: m.isUser
            ? BoxDecoration(
                color: c.layer01,
                border: Border.all(color: c.borderSubtle),
                borderRadius: BorderRadius.circular(TvRadius.small),
              )
            : null,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (m.isUser)
              SelectableText(m.content, style: TvType.bodyLong.copyWith(color: c.textPrimary))
            else
              MarkdownView(m.content),
            if (cites.isNotEmpty) ...[
              const SizedBox(height: TvSpace.s8),
              Text('REFERENCES', style: TvType.caption.copyWith(color: c.textSecondary)),
              for (final x in cites)
                InkWell(
                  key: Key('cite-${x.n}'),
                  onTap: () => unawaited(_open(context, x)),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: TvSpace.s2),
                    child: Text('[${x.n}] ${x.reference}', style: TvType.helper.copyWith(color: c.interactive)),
                  ),
                ),
            ],
            if (!m.isUser && m.id.isNotEmpty && m.content.isNotEmpty)
              Align(
                alignment: Alignment.centerRight,
                child: TvButton(
                  'Save to note',
                  key: Key('save-note-${m.id}'),
                  icon: TvIcon.add,
                  kind: TvButtonKind.ghost,
                  dense: true,
                  onPressed: () => unawaited(state.noteFromMessage(m.id)),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
