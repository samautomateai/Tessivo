import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../chat_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/approval.dart';
import '../../design/components/buttons.dart';
import '../../design/components/tags.dart';
import '../../design/components/tool_call.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/markdown_view.dart';

/// Where a tool call stands.
enum ToolStatus { running, waiting, done, denied, blocked }

/// One tool call of a reply, with its outcome so far.
class ToolRun {
  const ToolRun({required this.tool, required this.status, this.args = '', this.summary = '', this.by = ''});

  final String tool;
  final ToolStatus status;
  final String args;
  final String summary;

  /// Who let it run: "rule" or "you" (see ChatToolActivity.by).
  final String by;

  ToolRun _with(ToolStatus status, {String? summary, String? by}) =>
      ToolRun(tool: tool, status: status, args: args, summary: summary ?? this.summary, by: by ?? this.by);

  bool get open => status == ToolStatus.running || status == ToolStatus.waiting;

  /// The tag on its block and its word in the activity list.
  String get label => switch (status) {
    ToolStatus.done => by == 'rule' ? 'allow-listed' : (by == 'you' ? 'approved' : 'done'),
    ToolStatus.running => 'running',
    ToolStatus.waiting => 'waiting',
    ToolStatus.denied => 'denied',
    ToolStatus.blocked => 'blocked',
  };

  TvTagTone get tone => switch (status) {
    ToolStatus.done => TvTagTone.green,
    ToolStatus.waiting => TvTagTone.yellow,
    ToolStatus.running => TvTagTone.gray,
    ToolStatus.denied || ToolStatus.blocked => TvTagTone.red,
  };
}

/// Folds a reply's tool events into one run per call: a call opens a run,
/// its result, denial or block closes the latest open run of that tool, and
/// [waiting] marks the call that asks for approval.
List<ToolRun> toolRuns(List<ChatToolActivity> log, ChatApprovalNeeded? waiting) {
  final runs = <ToolRun>[];
  int openRun(String tool) => runs.lastIndexWhere((r) => r.tool == tool && r.open);
  for (final e in log) {
    final status = switch (e.kind) {
      'result' => ToolStatus.done,
      'denied' => ToolStatus.denied,
      'blocked' => ToolStatus.blocked,
      _ => null,
    };
    if (status == null) {
      runs.add(ToolRun(tool: e.tool, status: ToolStatus.running, args: e.args));
      continue;
    }
    final i = openRun(e.tool);
    if (i < 0) {
      runs.add(ToolRun(tool: e.tool, status: status, summary: e.summary, by: e.by));
    } else {
      runs[i] = runs[i]._with(status, summary: e.summary, by: e.by);
    }
  }
  if (waiting != null) {
    final i = openRun(waiting.tool);
    if (i < 0) {
      runs.add(ToolRun(tool: waiting.tool, status: ToolStatus.waiting, args: waiting.args));
    } else {
      runs[i] = runs[i]._with(ToolStatus.waiting);
    }
  }
  return runs;
}

/// The icon for a tool: a terminal for commands, the tool kit otherwise.
TvIcon toolIcon(String tool) => tool.contains('command') ? TvIcon.terminal : TvIcon.tools;

/// The conversation: stored turns, the reply's tool calls and approval right
/// after the question, then the reply streaming in.
class MessageList extends StatefulWidget {
  const MessageList({super.key, required this.chat, this.onSave});

  final ChatState chat;

  /// Saves a reply to the library; null hides the action.
  final void Function(ChatMessage)? onSave;

  @override
  State<MessageList> createState() => _MessageListState();
}

class _MessageListState extends State<MessageList> {
  final _scroll = ScrollController();

  @override
  void didUpdateWidget(MessageList old) {
    super.didUpdateWidget(old);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
    });
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  List<Widget> _tools(ChatState chat) {
    final a = chat.approval;
    return [
      for (final r in toolRuns(chat.toolLog, a))
        Padding(
          padding: const EdgeInsets.only(bottom: TvSpace.s8),
          child: ToolCallBlock(
            icon: toolIcon(r.tool),
            tool: r.tool,
            tags: [TvTag(r.label, tone: r.tone)],
            output: [if (r.args.isNotEmpty) r.args, if (r.summary.isNotEmpty) r.summary].join('\n'),
          ),
        ),
      if (a != null)
        Padding(
          padding: const EdgeInsets.only(bottom: TvSpace.s12),
          child: ApprovalCard(
            key: const Key('approval-card'),
            tool: a.tool,
            args: a.args,
            onAllow: () => unawaited(chat.answerTool(allow: true)),
            onAlways: () => unawaited(chat.answerTool(allow: true, remember: true)),
            onDeny: () => unawaited(chat.answerTool(allow: false)),
          ),
        ),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final chat = widget.chat;
    final messages = chat.open?.messages ?? const <ChatMessage>[];
    final streaming = chat.sending ? chat.streaming : null;
    if (messages.isEmpty && streaming == null) {
      return Center(
        child: Text(
          'Say hello, or attach files and ask about them.',
          style: TvType.body.copyWith(color: c.textSecondary),
        ),
      );
    }
    final asked = messages.lastIndexWhere((m) => m.isUser);
    final items = <Widget>[
      for (final (i, m) in messages.indexed) ...[
        if (m.role == 'note') _Note(m.content) else MessageBubble(message: m, onSave: widget.onSave),
        if (i == asked) ..._tools(chat),
      ],
      if (streaming != null)
        MessageBubble(
          key: const Key('chat-streaming'),
          message: ChatMessage(id: '', role: 'assistant', content: streaming.isEmpty ? '…' : streaming),
        ),
    ];
    return ListView(
      controller: _scroll,
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s16),
      children: [
        for (final item in items)
          Align(
            alignment: Alignment.topLeft,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: TvSize.readingWidth),
              child: item,
            ),
          ),
      ],
    );
  }
}

class _Note extends StatelessWidget {
  const _Note(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: TvSpace.s12),
    child: Text(text, style: TvType.helper.copyWith(color: TvColors.of(context).textHelper)),
  );
}

/// One turn: the question in a layer box, the reply as Markdown on the page,
/// as in VS Code's chat.
class MessageBubble extends StatelessWidget {
  const MessageBubble({super.key, required this.message, this.onSave});

  final ChatMessage message;
  final void Function(ChatMessage)? onSave;

  static String _chipLabel(ChatAttachment a) =>
      a.tokens > 0 ? '${a.display} · ≈${a.tokens ~/ 1000}k tokens' : a.display;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final m = message;
    final footer = [
      if (m.model != null && m.model!.isNotEmpty) m.model!,
      if (m.stopped) m.error == null ? 'stopped' : 'failed: ${m.error}',
    ].join(' · ');
    final canSave = !m.isUser && onSave != null && m.id.isNotEmpty && m.content.isNotEmpty;
    return Container(
      key: m.id.isEmpty ? null : Key('msg-${m.id}'),
      width: double.infinity,
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
          if (m.attachments.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: TvSpace.s8),
              child: Wrap(
                spacing: TvSpace.s4,
                runSpacing: TvSpace.s4,
                children: [for (final a in m.attachments) TvTag(_chipLabel(a), tone: TvTagTone.blue)],
              ),
            ),
          if (m.isUser)
            SelectableText(m.content, style: TvType.bodyLong.copyWith(color: c.textPrimary))
          else
            MarkdownView(m.content),
          if (footer.isNotEmpty || canSave)
            Row(
              children: [
                Expanded(
                  child: Text(footer, style: TvType.helper.copyWith(color: c.textHelper)),
                ),
                if (canSave)
                  TvButton(
                    'Save as output',
                    key: Key('save-${m.id}'),
                    icon: TvIcon.export,
                    kind: TvButtonKind.ghost,
                    dense: true,
                    onPressed: () => onSave!(m),
                  ),
              ],
            ),
        ],
      ),
    );
  }
}
