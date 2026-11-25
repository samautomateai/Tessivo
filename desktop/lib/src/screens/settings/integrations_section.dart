import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/menu.dart';
import '../../design/components/notices.dart';
import '../../design/components/table.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import 'integration_dialogs.dart';

/// Settings → Integrations: trackers, wikis and folders, with Add…, and for
/// each Set token…, Test and Remove (REP-4, REP-5).
class IntegrationsSection extends StatefulWidget {
  const IntegrationsSection({super.key, required this.state});

  final AppState state;

  @override
  State<IntegrationsSection> createState() => _IntegrationsSectionState();
}

class _IntegrationsSectionState extends State<IntegrationsSection> {
  CoreClient get core => widget.state.core;
  List<IntegrationInfo>? _list;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_run(() async {}));
  }

  /// Runs a call, reloads the list and shows [done] (or a refusal) as a toast.
  Future<void> _run(Future<void> Function() call, {String? done}) async {
    setState(() => _busy = true);
    try {
      await call();
      final list = await core.listIntegrations();
      if (!mounted) return;
      setState(() => _list = list);
      if (done != null) showTvToast(context, done, kind: TvNoticeKind.success);
    } on CoreException catch (e) {
      if (mounted) showTvToast(context, e.message, kind: TvNoticeKind.error);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _add() async {
    final got = await showDialog<IntegrationInfo>(context: context, builder: (_) => const AddIntegrationDialog());
    if (got == null) return;
    await _run(() => core.addIntegration(got), done: 'Added ${got.name}');
    if (got.tokenRef.isNotEmpty && mounted) await _setToken(got);
  }

  Future<void> _setToken(IntegrationInfo i) async {
    final value = await showDialog<String>(
      context: context,
      builder: (_) => TokenDialog(name: i.name),
    );
    if (value == null || value.isEmpty) return;
    await _run(() => core.setSecret(i.tokenRef, value), done: 'Token saved for ${i.name}');
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final list = _list;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Issues go to GitHub, GitLab or Jira; pages to Confluence, Notion or a folder.',
                style: TvType.body.copyWith(color: c.textSecondary),
              ),
            ),
            TvButton(
              'Add…',
              key: const Key('integration-add'),
              icon: TvIcon.add,
              dense: true,
              onPressed: _busy ? null : () => unawaited(_add()),
            ),
          ],
        ),
        const SizedBox(height: TvSpace.s16),
        if (list == null)
          const Center(child: CircularProgressIndicator())
        else if (list.isEmpty)
          Text(
            'No integrations yet.',
            key: const Key('no-integrations'),
            style: TvType.body.copyWith(color: c.textSecondary),
          )
        else
          TvDataTable(
            columns: const ['Name', 'Kind', 'Sends to', 'Token', ''],
            flex: const [2, 2, 4, 2, 1],
            rows: [for (final i in list) _row(c, i)],
          ),
      ],
    );
  }

  List<Widget> _row(TvColors c, IntegrationInfo i) => [
    Text(i.name),
    Text(i.kind),
    Tooltip(message: i.baseUrl, child: Text(i.target)),
    Text(
      i.kind == 'folder' ? '—' : (i.tokenSet ? 'Set' : 'Missing'),
      style: i.tokenSet || i.kind == 'folder' ? null : TextStyle(color: c.warning),
    ),
    Align(
      alignment: Alignment.centerRight,
      child: TvMenuButton(
        key: Key('integration-menu-${i.name}'),
        label: 'Actions',
        icon: TvIcon.more,
        compact: true,
        items: [
          if (i.kind != 'folder')
            TvMenuItem(
              'Set token…',
              key: Key('integration-token-${i.name}'),
              onSelected: () => unawaited(_setToken(i)),
            ),
          TvMenuItem(
            'Test',
            key: Key('integration-test-${i.name}'),
            onSelected: () => unawaited(_run(() => core.testIntegration(i.name), done: '${i.name}: connected')),
          ),
          TvMenuItem(
            'Remove',
            key: Key('integration-remove-${i.name}'),
            onSelected: () => unawaited(_run(() => core.removeIntegration(i.name))),
          ),
        ],
      ),
    ),
  ];
}
