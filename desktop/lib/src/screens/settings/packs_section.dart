import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/notices.dart';
import '../../design/components/table.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/markdown_view.dart';

/// Settings → Packs: the team packs added, with Add pack…, Refresh and
/// Remove; picking one shows its README and any problems.
class PacksSection extends StatefulWidget {
  const PacksSection({super.key, required this.state});

  final AppState state;

  @override
  State<PacksSection> createState() => _PacksSectionState();
}

class _PacksSectionState extends State<PacksSection> {
  CoreClient get core => widget.state.core;
  List<PackInfo>? _packs;
  String? _open;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_run(core.listPacks));
  }

  /// Runs a pack call, keeps its list, refreshes the MCP servers (packs bring
  /// some) and shows a refusal as a toast.
  Future<void> _run(Future<List<PackInfo>> Function() call) async {
    setState(() => _busy = true);
    try {
      final list = await call();
      if (mounted) setState(() => _packs = list);
      await widget.state.refreshMcpServers();
    } on CoreException catch (e) {
      if (mounted) showTvToast(context, e.message, kind: TvNoticeKind.error);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _add() async {
    final got = await showDialog<(String, String)>(context: context, builder: (_) => const _AddPackDialog());
    if (got == null || got.$1.isEmpty) return;
    await _run(() async {
      await core.addPack(got.$1, branch: got.$2);
      return core.listPacks();
    });
  }

  Future<void> _remove(String id) => _run(() async {
    await core.removePack(id);
    if (_open == id) _open = null;
    return core.listPacks();
  });

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final packs = _packs;
    final open = packs?.where((p) => p.id == _open).firstOrNull;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Pack MCP servers stay off until you trust each one in MCP servers.',
                style: TvType.body.copyWith(color: c.textSecondary),
              ),
            ),
            TvButton(
              'Refresh',
              key: const Key('packs-refresh'),
              icon: TvIcon.runAgain,
              dense: true,
              kind: TvButtonKind.ghost,
              onPressed: _busy ? null : () => unawaited(_run(core.refreshPacks)),
            ),
            const SizedBox(width: TvSpace.s8),
            TvButton(
              'Add pack…',
              key: const Key('pack-add'),
              icon: TvIcon.add,
              dense: true,
              onPressed: _busy ? null : () => unawaited(_add()),
            ),
          ],
        ),
        const SizedBox(height: TvSpace.s16),
        if (packs == null)
          const Center(child: CircularProgressIndicator())
        else if (packs.isEmpty)
          Text(
            'No packs yet. Add a folder, a .zip or a git repository your team shares.',
            key: const Key('no-packs'),
            style: TvType.body.copyWith(color: c.textSecondary),
          )
        else
          TvDataTable(
            columns: const ['Pack', 'Source', 'Updated', 'Items', ''],
            flex: const [3, 5, 2, 3, 1],
            onRowTap: (i) => setState(() => _open = packs[i].id == _open ? null : packs[i].id),
            rows: [for (final p in packs) _row(c, p)],
          ),
        if (open != null) ...[const SizedBox(height: TvSpace.s24), _Detail(pack: open)],
      ],
    );
  }

  List<Widget> _row(TvColors c, PackInfo p) => [
    Text(p.name),
    Tooltip(message: p.source, child: Text(p.source)),
    Text(p.kind == 'folder' ? 'live' : (p.fetchedAt.isEmpty ? '—' : p.fetchedAt.substring(0, 10))),
    Text(
      '${p.templates} templates · ${p.prompts} prompts · ${p.servers} servers'
      '${p.problems.isEmpty ? '' : ' · ${p.problems.length} problems'}',
      style: p.error.isNotEmpty || p.problems.isNotEmpty ? TextStyle(color: c.warning) : null,
    ),
    Align(
      alignment: Alignment.centerRight,
      child: TvIconButton(
        TvIcon.close,
        key: Key('pack-remove-${p.id}'),
        tooltip: 'Remove ${p.name}',
        onPressed: _busy ? null : () => unawaited(_remove(p.id)),
      ),
    ),
  ];
}

/// A pack's README and what it could not load.
class _Detail extends StatelessWidget {
  const _Detail({required this.pack});

  final PackInfo pack;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(pack.name, style: TvType.heading01.copyWith(color: c.textPrimary)),
        Text(
          [
            if (pack.publisher.isNotEmpty) pack.publisher,
            if (pack.version.isNotEmpty) 'v${pack.version}',
            pack.kind,
          ].join(' · '),
          style: TvType.helper.copyWith(color: c.textHelper),
        ),
        if (pack.error.isNotEmpty) ...[
          const SizedBox(height: TvSpace.s8),
          TvInlineNotice(kind: TvNoticeKind.warning, title: 'Last refresh failed', message: pack.error),
        ],
        for (final p in pack.problems) ...[
          const SizedBox(height: TvSpace.s8),
          TvInlineNotice(kind: TvNoticeKind.warning, title: 'Skipped', message: p),
        ],
        const SizedBox(height: TvSpace.s12),
        Container(
          key: const Key('pack-readme'),
          padding: const EdgeInsets.all(TvSpace.s16),
          decoration: BoxDecoration(
            color: c.layer01,
            border: Border.all(color: c.borderSubtle),
            borderRadius: BorderRadius.circular(TvRadius.small),
          ),
          child: pack.readme.isEmpty
              ? Text('This pack has no README.', style: TvType.body.copyWith(color: c.textSecondary))
              : MarkdownView(pack.readme),
        ),
      ],
    );
  }
}

class _AddPackDialog extends StatefulWidget {
  const _AddPackDialog();

  @override
  State<_AddPackDialog> createState() => _AddPackDialogState();
}

class _AddPackDialogState extends State<_AddPackDialog> {
  final _source = TextEditingController();
  final _branch = TextEditingController();

  @override
  void dispose() {
    _source.dispose();
    _branch.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add a pack'),
    content: SizedBox(
      width: 520,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TvTextField(
            label: 'Folder, .zip file or git URL',
            fieldKey: const Key('pack-source'),
            controller: _source,
            hint: 'https://github.com/acme/tessivo-pack.git',
            helper: 'Git repositories are fetched with your own git sign-in; Tessivo stores none.',
          ),
          const SizedBox(height: TvSpace.s12),
          TvTextField(label: 'Branch (git only, optional)', fieldKey: const Key('pack-branch'), controller: _branch),
        ],
      ),
    ),
    actions: [
      TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
      TvButton(
        'Add',
        key: const Key('pack-add-save'),
        kind: TvButtonKind.primary,
        onPressed: () => Navigator.of(context).pop((_source.text.trim(), _branch.text.trim())),
      ),
    ],
  );
}
