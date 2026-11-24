import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/menu.dart';
import '../../design/components/notices.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/chat_tools.dart';
import '../../widgets/text_prompt_dialog.dart';

/// Thinking effort choices; '' uses the app default.
const chatThinking = ['', 'off', 'low', 'medium', 'high'];

/// A rough size of what the next turn sends: the conversation and the
/// attachments waiting, at about four characters a token.
int chatTokens(ChatSession? s, List<ChatAttachment> pending) =>
    (s?.messages ?? const <ChatMessage>[]).fold(0, (n, m) => n + m.content.length) ~/ 4 +
    pending.fold(0, (n, a) => n + a.tokens);

/// The message box: attachments waiting, the text, and a toolbar with
/// attach, tools, model, thinking, the size and Send.
class Composer extends StatefulWidget {
  const Composer({super.key, required this.state, required this.models});

  final AppState state;

  /// The model aliases to pick from; '' is the default model.
  final List<String> models;

  @override
  State<Composer> createState() => _ComposerState();
}

class _ComposerState extends State<Composer> {
  final _text = TextEditingController();

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _text.text;
    final chat = widget.state.chat;
    if (text.trim().isEmpty || chat.sending) return;
    _text.clear();
    final sent = await chat.send(text);
    if (!sent && mounted && _text.text.isEmpty) _text.text = text; // give it back to fix and resend
  }

  Future<void> _attach(String kind) async {
    final url = kind == 'url';
    final ref = await askText(
      context,
      title: url ? 'Attach a web page' : 'Attach a file or folder',
      label: url ? 'URL (http or https)' : 'Absolute path',
      maxLength: url ? 2048 : 4096,
    );
    if (ref == null) return;
    try {
      url ? checkWebUrl(ref) : checkContextPath(ref);
      widget.state.chat.attach(ChatAttachment(kind: kind, ref: url ? ref.trim() : ref));
    } on FormatException catch (e) {
      if (mounted) showTvToast(context, e.message, kind: TvNoticeKind.error);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final state = widget.state;
    final chat = state.chat;
    final s = chat.open;
    final tools = s?.mcpServers.length ?? 0;
    return Container(
      key: const Key('composer'),
      margin: const EdgeInsets.fromLTRB(TvSpace.s24, 0, TvSpace.s24, TvSpace.s16),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border.all(color: c.borderSubtle),
        borderRadius: BorderRadius.circular(TvRadius.small),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (chat.pending.isNotEmpty)
            Padding(
              padding: const EdgeInsets.fromLTRB(TvSpace.s8, TvSpace.s8, TvSpace.s8, 0),
              child: Wrap(
                spacing: TvSpace.s4,
                runSpacing: TvSpace.s4,
                children: [
                  for (final a in chat.pending)
                    Row(
                      key: Key('pending-${a.ref}'),
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        TvTag(a.display, tone: TvTagTone.blue),
                        TvIconButton(TvIcon.close, tooltip: 'Remove ${a.display}', onPressed: () => chat.detach(a)),
                      ],
                    ),
                ],
              ),
            ),
          TextField(
            key: const Key('chat-input'),
            controller: _text,
            minLines: 1,
            maxLines: 8,
            style: TvType.bodyLong.copyWith(color: c.textPrimary),
            cursorColor: c.interactive,
            decoration: tvFieldDecoration(c, hint: 'Ask anything, or attach files and ask about them').copyWith(
              fillColor: c.layer01,
              border: InputBorder.none,
              enabledBorder: InputBorder.none,
              focusedBorder: InputBorder.none,
              contentPadding: const EdgeInsets.all(TvSpace.s12),
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(TvSpace.s4, 0, TvSpace.s8, TvSpace.s8),
            child: Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: TvSpace.s4,
              runSpacing: TvSpace.s4,
              children: [
                TvMenuButton(
                  key: const Key('chat-attach'),
                  label: 'Attach',
                  icon: TvIcon.attach,
                  items: [
                    TvMenuItem('File or folder…', onSelected: () => unawaited(_attach('path'))),
                    TvMenuItem(
                      'Web page…',
                      key: const Key('chat-attach-url'),
                      onSelected: () => unawaited(_attach('url')),
                    ),
                    for (final nb in state.notebooks)
                      TvMenuItem(
                        'Notebook: ${nb.name}',
                        onSelected: () => chat.attach(ChatAttachment(kind: 'notebook', ref: nb.id, label: nb.name)),
                      ),
                  ],
                ),
                TvToolButton(
                  'Tools · $tools',
                  key: const Key('chat-tools'),
                  icon: TvIcon.tools,
                  onPressed: chat.sending || s == null ? null : () => unawaited(pickChatTools(context, state)),
                ),
                if (s != null) ...[
                  _InlineSelect(
                    key: const Key('chat-model'),
                    value: s.model,
                    items: [
                      for (final m in {...widget.models, s.model}) (m, m.isEmpty ? 'Default model' : m),
                    ],
                    onChanged: chat.sending ? null : (m) => unawaited(chat.setModel(m)),
                  ),
                  _InlineSelect(
                    key: const Key('chat-thinking'),
                    value: chatThinking.contains(s.thinkingEffort) ? s.thinkingEffort : '',
                    items: [for (final t in chatThinking) (t, t.isEmpty ? 'Default thinking' : 'Thinking: $t')],
                    onChanged: chat.sending ? null : (t) => unawaited(chat.setThinking(t)),
                  ),
                ],
                _Trailing(
                  tokens: chatTokens(s, chat.pending),
                  send: chat.sending
                      ? TvButton('Stop', key: const Key('chat-stop'), icon: TvIcon.close, onPressed: chat.stop)
                      : TvButton(
                          'Send',
                          key: const Key('chat-send'),
                          icon: TvIcon.send,
                          kind: TvButtonKind.primary,
                          onPressed: () => unawaited(_send()),
                        ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Trailing extends StatelessWidget {
  const _Trailing({required this.tokens, required this.send});

  final int tokens;
  final Widget send;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text('≈ ${tokens ~/ 1000}k tokens', style: TvType.helper.copyWith(color: TvColors.of(context).textHelper)),
      const SizedBox(width: TvSpace.s8),
      send,
    ],
  );
}

/// A borderless select for the composer's toolbar.
class _InlineSelect extends StatelessWidget {
  const _InlineSelect({super.key, required this.value, required this.items, required this.onChanged});

  final String value;
  final List<(String, String)> items;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final change = onChanged;
    return SizedBox(
      height: TvSize.button,
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: value,
          isDense: true,
          dropdownColor: c.layer02,
          borderRadius: BorderRadius.circular(TvRadius.medium),
          iconEnabledColor: c.textSecondary,
          style: TvType.body.copyWith(color: c.textSecondary),
          focusColor: c.layer01,
          padding: const EdgeInsets.symmetric(horizontal: TvSpace.s8),
          items: [for (final (v, text) in items) DropdownMenuItem(value: v, child: Text(text))],
          onChanged: change == null
              ? null
              : (v) {
                  if (v != null) change(v);
                },
        ),
      ),
    );
  }
}
