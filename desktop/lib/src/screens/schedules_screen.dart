import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/empty_state.dart';
import '../design/components/fields.dart';
import '../design/components/headers.dart';
import '../design/components/notices.dart';
import '../design/components/table.dart';
import '../design/components/tags.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// Missed-run policies the core accepts.
const missedPolicies = ['skip', 'run_once'];

/// Lists, adds, toggles and removes schedules for the selected notebook.
/// Schedules run in the core while the app (or `tess serve`) is active.
class SchedulesScreen extends StatefulWidget {
  const SchedulesScreen({super.key, required this.state});

  final AppState state;

  @override
  State<SchedulesScreen> createState() => _SchedulesScreenState();
}

class _SchedulesScreenState extends State<SchedulesScreen> {
  final _cron = TextEditingController();
  final _prompt = TextEditingController();
  String _missed = missedPolicies.first;
  List<ScheduleInfo> _items = const [];
  String? _loadedFor;
  String? _error;
  bool _busy = false;

  String? get _notebookId => widget.state.selectedId;

  @override
  void initState() {
    super.initState();
    unawaited(_reload());
  }

  @override
  void didUpdateWidget(SchedulesScreen old) {
    super.didUpdateWidget(old);
    if (_loadedFor != _notebookId) unawaited(_reload());
  }

  @override
  void dispose() {
    _cron.dispose();
    _prompt.dispose();
    super.dispose();
  }

  Future<void> _guard(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } on Object catch (e) {
      _error = e is FormatException ? e.message : '$e';
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _reload() async {
    final id = _notebookId;
    _loadedFor = id;
    if (id == null) return;
    await _guard(() async {
      final items = await widget.state.core.listSchedules(id);
      if (_loadedFor == id) _items = items;
    });
  }

  Future<void> _add() async {
    final id = _notebookId;
    if (id == null) return;
    await _guard(() async {
      final s = await widget.state.core.addSchedule(
        id,
        cron: _cron.text.trim(),
        prompt: _prompt.text.trim(),
        missed: _missed,
      );
      _items = [..._items, s];
      _cron.clear();
      _prompt.clear();
    });
  }

  Future<void> _toggle(ScheduleInfo s, bool enabled) async {
    final id = _notebookId!;
    await _guard(() async {
      final updated = await widget.state.core.setScheduleEnabled(id, s.id, enabled: enabled);
      _items = [for (final x in _items) x.id == s.id ? updated : x];
    });
  }

  Future<void> _delete(ScheduleInfo s) async {
    final id = _notebookId!;
    await _guard(() async {
      await widget.state.core.deleteSchedule(id, s.id);
      _items = [
        for (final x in _items)
          if (x.id != s.id) x,
      ];
    });
  }

  List<Widget> _row(ScheduleInfo s) {
    final c = TvColors.of(context);
    final last = s.history.isEmpty ? null : s.history.last;
    final tone = switch (last?.status) {
      'ok' => TvTagTone.green,
      'failed' => TvTagTone.red,
      _ => TvTagTone.yellow,
    };
    return [
      Text(
        s.cron,
        key: Key('schedule-${s.id}'),
        style: TvType.code.copyWith(color: c.textPrimary),
      ),
      Tooltip(message: s.prompt, child: Text(s.prompt.isEmpty ? 'The notebook prompt' : s.prompt)),
      Text(s.nextRun ?? '—'),
      last == null
          ? const Text('—')
          : Tooltip(
              message: last.error ?? last.status,
              child: TvTag(last.status.replaceAll('_', ' '), tone: tone),
            ),
      Text(s.missed == 'run_once' ? 'Run once' : 'Skip'),
      Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          SizedBox(
            height: 20,
            child: FittedBox(
              child: Switch(
                key: Key('schedule-enabled-${s.id}'),
                value: s.enabled,
                activeTrackColor: c.success,
                onChanged: _busy ? null : (v) => unawaited(_toggle(s, v)),
              ),
            ),
          ),
          TvIconButton(
            TvIcon.close,
            key: Key('schedule-delete-${s.id}'),
            tooltip: 'Remove',
            onPressed: _busy ? null : () => unawaited(_delete(s)),
          ),
        ],
      ),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final nb = widget.state.selected;
    if (nb == null) {
      return ColoredBox(
        color: c.background,
        child: TvEmptyState(
          icon: TvIcon.schedules,
          title: 'No notebook open',
          message: 'Schedules belong to a notebook. Open one, then come back to schedule its runs.',
          actionLabel: 'Go to notebooks',
          onAction: () => widget.state.go(Section.notebooks),
        ),
      );
    }
    return ColoredBox(
      color: c.background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TvPageHeader(title: 'Schedules', meta: nb.name),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s8, TvSpace.s24, TvSpace.s32),
              children: [
                Text(
                  'Runs of "${nb.name}" while Tessivo (or tess serve) is running.',
                  style: TvType.helper.copyWith(color: c.textHelper),
                ),
                if (_error case final e?) ...[
                  const SizedBox(height: TvSpace.s12),
                  TvInlineNotice(key: const Key('schedule-error'), kind: TvNoticeKind.error, title: e),
                ],
                const SizedBox(height: TvSpace.s16),
                if (_items.isEmpty && !_busy)
                  Text(
                    'No schedules yet. Add one below, for example every Monday at 08:00.',
                    key: const Key('no-schedules'),
                    style: TvType.body.copyWith(color: c.textSecondary),
                  )
                else
                  TvDataTable(
                    key: const Key('schedules-table'),
                    columns: const ['When', 'Prompt', 'Next run', 'Last result', 'If missed', ''],
                    flex: const [3, 5, 3, 2, 2, 2],
                    rows: [for (final s in _items) _row(s)],
                  ),
                const SizedBox(height: TvSpace.s32),
                Align(alignment: Alignment.topLeft, child: _addForm(c)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _addForm(TvColors c) => ConstrainedBox(
    constraints: const BoxConstraints(maxWidth: TvSize.readingWidth),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('Add a schedule', style: TvType.heading01.copyWith(color: c.textPrimary)),
        const SizedBox(height: TvSpace.s12),
        TvTextField(
          label: 'When (cron)',
          fieldKey: const Key('schedule-cron'),
          controller: _cron,
          hint: '0 8 * * MON  or  @daily',
          mono: true,
        ),
        const SizedBox(height: TvSpace.s12),
        TvTextField(
          label: 'Prompt',
          fieldKey: const Key('schedule-prompt'),
          controller: _prompt,
          helper: 'Optional; the notebook prompt when empty.',
        ),
        const SizedBox(height: TvSpace.s12),
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            SizedBox(
              width: 220,
              child: TvDropdown<String>(
                key: const Key('schedule-missed'),
                label: 'If a run was missed',
                items: const [('skip', 'Skip it'), ('run_once', 'Run once')],
                value: _missed,
                onChanged: (v) => setState(() => _missed = v),
              ),
            ),
            const Spacer(),
            TvButton(
              'Add schedule',
              key: const Key('schedule-add'),
              icon: TvIcon.add,
              kind: TvButtonKind.primary,
              onPressed: _busy ? null : () => unawaited(_add()),
            ),
          ],
        ),
      ],
    ),
  );
}
