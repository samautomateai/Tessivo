import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';

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
    _reload();
  }

  @override
  void didUpdateWidget(SchedulesScreen old) {
    super.didUpdateWidget(old);
    if (_loadedFor != _notebookId) _reload();
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

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final nb = widget.state.selected;
    return ListView(
      padding: const EdgeInsets.all(28),
      children: [
        Text('Schedules', style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w600)),
        const SizedBox(height: 4),
        Text(
          nb == null ? 'Select a notebook first.' : 'Runs for "${nb.name}" while Tessivo is open.',
          style: const TextStyle(color: Tokens.textMuted),
        ),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(
            _error!,
            key: const Key('schedule-error'),
            style: const TextStyle(color: Colors.red),
          ),
        ],
        const SizedBox(height: 16),
        if (nb != null) ...[
          for (final s in _items) _ScheduleCard(schedule: s, busy: _busy, onToggle: _toggle, onDelete: _delete),
          if (_items.isEmpty && !_busy) const Text('No schedules yet.'),
          const SizedBox(height: 20),
          _addForm(text),
        ],
      ],
    );
  }

  Widget _addForm(TextTheme text) => Card(
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Add schedule', style: text.titleSmall),
          const SizedBox(height: 8),
          TextField(
            key: const Key('schedule-cron'),
            controller: _cron,
            decoration: const InputDecoration(labelText: 'Cron', hintText: '0 8 * * MON  or  @daily'),
          ),
          TextField(
            key: const Key('schedule-prompt'),
            controller: _prompt,
            decoration: const InputDecoration(labelText: 'Prompt (optional, defaults to the notebook prompt)'),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              const Text('If missed: '),
              DropdownButton<String>(
                key: const Key('schedule-missed'),
                value: _missed,
                items: [for (final p in missedPolicies) DropdownMenuItem(value: p, child: Text(p))],
                onChanged: (v) => setState(() => _missed = v ?? _missed),
              ),
              const Spacer(),
              FilledButton(key: const Key('schedule-add'), onPressed: _busy ? null : _add, child: const Text('Add')),
            ],
          ),
        ],
      ),
    ),
  );
}

class _ScheduleCard extends StatelessWidget {
  const _ScheduleCard({required this.schedule, required this.busy, required this.onToggle, required this.onDelete});

  final ScheduleInfo schedule;
  final bool busy;
  final void Function(ScheduleInfo, bool) onToggle;
  final void Function(ScheduleInfo) onDelete;

  @override
  Widget build(BuildContext context) {
    final s = schedule;
    final last = s.history.isEmpty ? null : s.history.last;
    final details = [
      'next: ${s.nextRun ?? '—'}',
      'missed: ${s.missed}',
      if (last != null) 'last: ${last.status}${last.error == null ? '' : ' (${last.error})'}',
    ].join(' · ');
    return Card(
      child: ListTile(
        key: Key('schedule-${s.id}'),
        title: Text(s.cron, style: const TextStyle(fontFamily: 'monospace')),
        subtitle: Text(s.prompt.isEmpty ? details : '${s.prompt}\n$details'),
        isThreeLine: s.prompt.isNotEmpty,
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Switch(
              key: Key('schedule-enabled-${s.id}'),
              value: s.enabled,
              onChanged: busy ? null : (v) => onToggle(s, v),
            ),
            IconButton(
              key: Key('schedule-delete-${s.id}'),
              tooltip: 'Remove',
              icon: const Icon(Icons.delete_outline),
              onPressed: busy ? null : () => onDelete(s),
            ),
          ],
        ),
      ),
    );
  }
}
