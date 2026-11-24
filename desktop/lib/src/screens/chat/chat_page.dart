import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../chat_state.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/empty_state.dart';
import '../../design/components/headers.dart';
import '../../design/components/notices.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../chat_dialogs.dart';
import 'composer.dart';
import 'conversation.dart';

/// The chat page: the open conversation with its tool calls and approvals in
/// place, and the message box. The list is in the side bar, "In this chat"
/// in the panel.
class ChatPage extends StatefulWidget {
  const ChatPage({super.key, required this.state});

  final AppState state;

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  ChatState get chat => widget.state.chat;
  List<String> _models = const [''];

  @override
  void initState() {
    super.initState();
    unawaited(chat.load());
    unawaited(_loadModels());
  }

  Future<void> _loadModels() async {
    try {
      final s = await widget.state.core.getSettings();
      if (mounted) setState(() => _models = ['', for (final a in s.aliases) a.name]);
    } on Object {
      // The default model still works without the alias list.
    }
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: chat,
    builder: (context, _) {
      final c = TvColors.of(context);
      final s = chat.open;
      if (s == null) {
        return ColoredBox(
          color: c.background,
          child: TvEmptyState(
            icon: TvIcon.chat,
            title: 'Start a chat',
            message: 'Ask anything, attach files or notebooks, and keep the good answers.',
            actionLabel: 'New chat',
            actionKey: const Key('chat-start'),
            onAction: () => unawaited(chat.newChat()),
          ),
        );
      }
      return ColoredBox(
        color: c.background,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            LayoutBuilder(
              builder: (context, box) => TvPageHeader(
                title: s.title.isEmpty ? 'New chat' : s.title,
                meta: s.model.isEmpty ? null : s.model,
                actions: [
                  TvToolButton(
                    'Make this repeatable',
                    key: const Key('chat-repeat'),
                    icon: TvIcon.templates,
                    compact: box.maxWidth < 520,
                    onPressed: chat.sending || s.messages.isEmpty
                        ? null
                        : () => unawaited(showRepeatDialog(context, widget.state)),
                  ),
                ],
              ),
            ),
            if (chat.error case final e?)
              Padding(
                padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s12, TvSpace.s24, 0),
                child: TvInlineNotice(
                  key: const Key('chat-error'),
                  kind: TvNoticeKind.error,
                  title: e,
                  action: TvButton('Dismiss', dense: true, kind: TvButtonKind.ghost, onPressed: chat.clearError),
                ),
              ),
            const SizedBox(height: TvSpace.s8),
            Expanded(
              child: MessageList(
                chat: chat,
                onSave: chat.sending ? null : (m) => unawaited(showSaveReplyDialog(context, chat, m)),
              ),
            ),
            Composer(state: widget.state, models: _models),
          ],
        ),
      );
    },
  );
}
