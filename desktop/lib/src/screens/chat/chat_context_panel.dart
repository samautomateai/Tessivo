import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/headers.dart';
import '../../design/components/rows.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/chat_tools.dart';
import 'conversation.dart';

TvIcon _attachmentIcon(ChatAttachment a) => switch (a.kind) {
  'url' => TvIcon.link,
  'notebook' => TvIcon.notebooks,
  'output' => TvIcon.document,
  _ => TvIcon.folder,
};

/// The secondary panel on the chat page, "In this chat": what the chat has
/// read, the tools it may use, and the current reply's tool activity.
class ChatContextPanel extends StatelessWidget {
  const ChatContextPanel({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    key: const Key('chat-context'),
    listenable: state.chat,
    builder: (context, _) {
      final c = TvColors.of(context);
      final chat = state.chat;
      final s = chat.open;
      final seen = <String>{};
      final attachments = [
        for (final m in s?.messages ?? const <ChatMessage>[])
          for (final a in m.attachments)
            if (seen.add('${a.kind}:${a.ref}')) a,
        for (final a in chat.pending)
          if (seen.add('${a.kind}:${a.ref}')) a,
      ];
      final runs = toolRuns(chat.toolLog, chat.approval);
      final muted = TvType.helper.copyWith(color: c.textHelper);
      Widget note(String text) => Padding(
        padding: const EdgeInsets.fromLTRB(TvSpace.s16, 0, TvSpace.s16, TvSpace.s8),
        child: Text(text, style: muted),
      );
      return ListView(
        children: [
          const TvPanelHeader('In this chat'),
          const _Section('Context'),
          if (attachments.isEmpty) note('Nothing attached. Attach files, web pages or notebooks from the message box.'),
          for (final a in attachments)
            TvTreeRow(
              leading: TvIconView(_attachmentIcon(a), size: 14),
              label: a.display,
              trailing: a.tokens > 0 ? '${a.tokens ~/ 1000}k' : null,
            ),
          _Section(
            'Tools',
            action: TvIconButton(
              TvIcon.edit,
              key: const Key('chat-context-tools'),
              tooltip: 'Choose tools',
              onPressed: s == null || chat.sending ? null : () => unawaited(pickChatTools(context, state)),
            ),
          ),
          if ((s?.mcpServers ?? const []).isEmpty) note('No tools. Choose MCP servers the model may use.'),
          for (final name in s?.mcpServers ?? const <String>[])
            TvTreeRow(leading: const TvIconView(TvIcon.tools, size: 14), label: name),
          const _Section('Activity'),
          if (runs.isEmpty) note('Tool calls of the reply show here.'),
          for (final r in runs)
            TvTreeRow(leading: TvIconView(toolIcon(r.tool), size: 14), label: r.tool, trailing: r.label),
        ],
      );
    },
  );
}

class _Section extends StatelessWidget {
  const _Section(this.title, {this.action});

  final String title;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(TvSpace.s16, TvSpace.s12, TvSpace.s8, TvSpace.s4),
    child: SizedBox(
      height: TvSize.buttonDense,
      child: Row(
        children: [
          Expanded(
            child: Text(title.toUpperCase(), style: TvType.caption.copyWith(color: TvColors.of(context).textSecondary)),
          ),
          if (action case final a?) SizedBox.square(dimension: 24, child: FittedBox(child: a)),
        ],
      ),
    ),
  );
}
