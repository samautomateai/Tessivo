import 'package:tessivo_desktop/src/core/models.dart';

/// Schedules for the demo core, per notebook.
mixin DemoSchedules {
  int _nextScheduleId = 100;
  final Map<String, List<ScheduleInfo>> _schedules = {
    'nb-4': const [
      ScheduleInfo(
        id: 's1',
        cron: '0 8 * * MON',
        missed: 'run_once',
        enabled: true,
        prompt: "Produce this week's ops report",
        nextRun: 'Mon 08:00',
        history: [ScheduleRun(status: 'ok', startedAt: '2026-11-16 08:00')],
      ),
    ],
  };

  Future<List<ScheduleInfo>> listSchedules(String notebookId) async =>
      List.unmodifiable(_schedules[notebookId] ?? const <ScheduleInfo>[]);

  Future<ScheduleInfo> addSchedule(
    String notebookId, {
    required String cron,
    String prompt = '',
    String missed = 'skip',
  }) async {
    if (cron.trim().split(RegExp(r'\s+')).length != 5 && !cron.startsWith('@')) {
      throw const FormatException('want 5 fields (minute hour day month weekday)');
    }
    final s = ScheduleInfo(
      id: 's${_nextScheduleId++}',
      cron: cron,
      missed: missed,
      enabled: true,
      prompt: prompt,
      nextRun: 'soon',
    );
    _schedules[notebookId] = [...?_schedules[notebookId], s];
    return s;
  }

  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled}) async {
    final list = [...?_schedules[notebookId]];
    final i = list.indexWhere((s) => s.id == scheduleId);
    if (i < 0) throw StateError('unknown schedule $scheduleId');
    list[i] = list[i].copyWith(enabled: enabled);
    _schedules[notebookId] = list;
    return list[i];
  }

  Future<void> deleteSchedule(String notebookId, String scheduleId) async {
    _schedules[notebookId] = [...?_schedules[notebookId]]..removeWhere((s) => s.id == scheduleId);
  }
}
