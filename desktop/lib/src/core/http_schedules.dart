part of 'http_core_client.dart';

/// Schedule endpoints of the local API (docs/api.md).
mixin _HttpSchedules on _HttpChat {
  String _schedulesPath(String notebookId) => '/v1/notebooks/${Uri.encodeComponent(notebookId)}/schedules';

  Future<List<ScheduleInfo>> listSchedules(String notebookId) async {
    final json = await _json('GET', _schedulesPath(notebookId));
    return jsonList(json['schedules']).map(scheduleFromJson).toList();
  }

  Future<ScheduleInfo> addSchedule(
    String notebookId, {
    required String cron,
    String prompt = '',
    String missed = 'skip',
  }) async {
    final body = {'cron': cron, 'prompt': prompt, 'missed': missed};
    return scheduleFromJson(await _json('POST', _schedulesPath(notebookId), body: body));
  }

  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled}) async {
    final path = '${_schedulesPath(notebookId)}/${Uri.encodeComponent(scheduleId)}';
    return scheduleFromJson(await _json('PATCH', path, body: {'enabled': enabled}));
  }

  Future<void> deleteSchedule(String notebookId, String scheduleId) async {
    await _json('DELETE', '${_schedulesPath(notebookId)}/${Uri.encodeComponent(scheduleId)}');
  }
}
