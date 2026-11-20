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
