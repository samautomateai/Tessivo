/// A notification-worthy event from the core (e.g. a scheduled run finished).
class CoreEvent {
  const CoreEvent({
    required this.kind,
    required this.notebookName,
    this.notebookId = '',
    this.sourceId,
    this.title,
    this.error,
    this.tool,
    this.ran = false,
  });
  final String kind;
  final String notebookName;
  final String notebookId;

  /// The source a research summary event is about.
  final String? sourceId;
  final String? title;
  final String? error;
  final String? tool;

  /// A watched notebook's change started a run (sources.changed in run mode).
  final bool ran;

  /// Whether the event changes the list of pending approvals.
  bool get isRunEvent => kind.startsWith('run.');

  /// A research notebook's summary started, is ready or failed.
  bool get isResearchEvent => kind.startsWith('research.');

  /// Short message for an in-app notification.
  String get message => switch (kind) {
    'schedule.finished' => '$notebookName: ${title ?? 'scheduled run'} is ready',
    'schedule.failed' => '$notebookName: scheduled run failed${error == null ? '' : ' ($error)'}',
    'schedule.skipped' => '$notebookName: missed run skipped',
    'schedule.paused' || 'run.approval_needed' => '$notebookName: approval needed${tool == null ? '' : ' for $tool'}',
    'run.finished' => '$notebookName: agent run finished',
    'run.failed' => '$notebookName: agent run failed${error == null ? '' : ' ($error)'}',
    'run.cancelled' => '$notebookName: agent run cancelled',
    'usage.warning' || 'usage.limit' => 'Token usage: ${error ?? 'limit reached'}',
    'sources.changed' =>
      '$notebookName: sources changed${ran
          ? ', a run started'
          : error == null
          ? ''
          : ' ($error)'}',
    'research.summary.failed' => '$notebookName: a summary failed${error == null ? '' : ' ($error)'}',
    _ => '$notebookName: scheduled run started',
  };
}
