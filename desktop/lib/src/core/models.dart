/// Plain data types the UI works with. They mirror the gRPC contract in
/// api/proto/tessivo/v1/tessivo.proto; the gRPC client maps into them so
/// widgets never depend on generated code.
library;

enum WorkTypeId { analyze, summarize, document, report }

class WorkType {
  const WorkType({
    required this.id,
    required this.name,
    required this.question,
    required this.starterPrompt,
    required this.suggestedContext,
  });

  final WorkTypeId id;
  final String name;
  final String question;
  final String starterPrompt;
  final List<String> suggestedContext;
}

class ContextItem {
  const ContextItem({
    required this.id,
    required this.label,
    required this.kind,
    required this.enabled,
    required this.estimatedTokens,
  });

  final String id;
  final String label;
  final String kind;
  final bool enabled;
  final int estimatedTokens;

  ContextItem copyWith({bool? enabled}) => ContextItem(
    id: id,
    label: label,
    kind: kind,
    enabled: enabled ?? this.enabled,
    estimatedTokens: estimatedTokens,
  );
}

class Notebook {
  const Notebook({
    required this.id,
    required this.name,
    required this.workType,
    required this.context,
    this.schedule,
  });

  final String id;
  final String name;
  final WorkTypeId workType;
  final List<ContextItem> context;
  final String? schedule;

  int get enabledTokens => context.where((c) => c.enabled).fold(0, (sum, c) => sum + c.estimatedTokens);

  Notebook copyWith({List<ContextItem>? context}) =>
      Notebook(id: id, name: name, workType: workType, context: context ?? this.context, schedule: schedule);
}

class OutputSummary {
  const OutputSummary({
    required this.id,
    required this.title,
    required this.workType,
    required this.notebookName,
    required this.trigger,
    required this.createdAt,
    required this.versions,
  });

  final String id;
  final String title;
  final WorkTypeId workType;
  final String notebookName;
  final String trigger;
  final String createdAt;
  final int versions;
}

/// A finished run as the UI shows it.
class RunOutcome {
  const RunOutcome({
    required this.valid,
    required this.modelUsed,
    required this.totalTokens,
    required this.markdown,
    this.fallbackNote,
  });

  final bool valid;
  final String modelUsed;
  final int totalTokens;
  final String markdown;
  final String? fallbackNote;
}

/// Progress events streamed during a run.
sealed class RunEvent {
  const RunEvent();
}

class RunStatus extends RunEvent {
  const RunStatus(this.text);
  final String text;
}

class RunDelta extends RunEvent {
  const RunDelta(this.text);
  final String text;
}

class RunFinished extends RunEvent {
  const RunFinished(this.outcome);
  final RunOutcome outcome;
}

/// One past scheduled run.
class ScheduleRun {
  const ScheduleRun({required this.status, required this.startedAt, this.error});
  final String status; // ok, failed, skipped
  final String startedAt;
  final String? error;
}

/// A notebook schedule as the UI shows it.
class ScheduleInfo {
  const ScheduleInfo({
    required this.id,
    required this.cron,
    required this.missed,
    required this.enabled,
    this.prompt = '',
    this.nextRun,
    this.history = const [],
  });

  final String id;
  final String cron;
  final String missed; // skip or run_once
  final bool enabled;
  final String prompt;
  final String? nextRun;
  final List<ScheduleRun> history;

  ScheduleInfo copyWith({bool? enabled}) => ScheduleInfo(
    id: id,
    cron: cron,
    missed: missed,
    enabled: enabled ?? this.enabled,
    prompt: prompt,
    nextRun: (enabled ?? this.enabled) ? nextRun : null,
    history: history,
  );
}

/// A notification-worthy event from the core (e.g. a scheduled run finished).
class CoreEvent {
  const CoreEvent({required this.kind, required this.notebookName, this.title, this.error});
  final String kind;
  final String notebookName;
  final String? title;
  final String? error;

  /// Short message for an in-app notification.
  String get message => switch (kind) {
    'schedule.finished' => '$notebookName: ${title ?? 'scheduled run'} is ready',
    'schedule.failed' => '$notebookName: scheduled run failed${error == null ? '' : ' ($error)'}',
    'schedule.skipped' => '$notebookName: missed run skipped',
    _ => '$notebookName: scheduled run started',
  };
}
