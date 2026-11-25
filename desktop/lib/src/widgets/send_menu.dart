import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/components/menu.dart';
import '../design/components/notices.dart';
import '../design/icons.dart';
import 'issues_dialog.dart';
import 'publish_dialog.dart';

/// "Send": file an Analyze output's findings as issues, or publish the output
/// to a folder, Confluence or Notion (WT-5, REP-4, REP-5).
class SendMenu extends StatelessWidget {
  const SendMenu({
    super.key,
    required this.state,
    required this.notebookId,
    required this.outputId,
    required this.workType,
    this.compact = false,
  });

  final AppState state;
  final String notebookId;
  final String outputId;
  final WorkTypeId? workType;
  final bool compact;

  Future<void> _open(BuildContext context, Widget dialog) async {
    final message = await showDialog<String>(context: context, builder: (_) => dialog);
    if (message != null && context.mounted) showTvToast(context, message, kind: TvNoticeKind.success);
  }

  @override
  Widget build(BuildContext context) => TvMenuButton(
    key: const Key('send'),
    label: 'Send',
    icon: TvIcon.send,
    compact: compact,
    items: [
      TvMenuItem(
        'Create issues…',
        key: const Key('send-issues'),
        enabled: workType == WorkTypeId.analyze,
        onSelected: () =>
            unawaited(_open(context, IssuesDialog(state: state, notebookId: notebookId, outputId: outputId))),
      ),
      TvMenuItem(
        'Publish…',
        key: const Key('send-publish'),
        onSelected: () =>
            unawaited(_open(context, PublishDialog(state: state, notebookId: notebookId, outputId: outputId))),
      ),
    ],
  );
}

/// Shown when no integration of the needed kind exists yet.
class NoIntegrations extends StatelessWidget {
  const NoIntegrations({super.key, required this.state, required this.what});

  final AppState state;

  /// What is missing, e.g. "GitHub, GitLab or Jira".
  final String what;

  @override
  Widget build(BuildContext context) => TvInlineNotice(
    kind: TvNoticeKind.info,
    title: 'Nothing to send to yet',
    message: 'Add $what in Settings → Integrations.',
    action: TextButton(
      key: const Key('open-integrations'),
      onPressed: () {
        Navigator.of(context).pop();
        state.openSettings(SettingsSection.integrations);
      },
      child: const Text('Open Integrations'),
    ),
  );
}
