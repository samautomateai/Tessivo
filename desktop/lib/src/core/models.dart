/// Plain data types the UI works with. They mirror the gRPC contract in
/// api/proto/tessivo/v1/tessivo.proto; the gRPC client maps into them so
/// widgets never depend on generated code.
library;

export 'output_detail.dart';

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

  /// True for a resource read from an MCP server at run time.
  bool get isMcp => kind.startsWith('MCP');

  /// Read when a run starts (an MCP resource or a web page), not from disk.
  bool get readAtRunTime => isMcp || kind == 'Web page' || kind == 'Git changes';

  ContextItem copyWith({bool? enabled}) =>
      ContextItem(id: id, label: label, kind: kind, enabled: enabled ?? this.enabled, estimatedTokens: estimatedTokens);
}

class Notebook {
  const Notebook({
    required this.id,
    required this.name,
    required this.workType,
    required this.context,
    this.schedule,
    this.memory = const [],
    this.mcpServers = const [],
    this.archived = false,
  });

  final String id;
  final String name;
  final WorkTypeId workType;
  final List<ContextItem> context;
  final String? schedule;

  /// MCP servers whose tools this notebook's agent runs may use.
  final List<String> mcpServers;

  /// Short facts the notebook keeps across runs (≤ 50 items, ≤ 1000 chars each).
  final List<String> memory;

  /// Archived notebooks are listed apart and their schedules do not run.
  final bool archived;

  int get enabledTokens => context.where((c) => c.enabled).fold(0, (sum, c) => sum + c.estimatedTokens);

  Notebook copyWith({
    List<ContextItem>? context,
    List<String>? memory,
    List<String>? mcpServers,
    String? name,
    bool? archived,
  }) => Notebook(
    id: id,
    name: name ?? this.name,
    workType: workType,
    context: context ?? this.context,
    schedule: schedule,
    memory: memory ?? this.memory,
    mcpServers: mcpServers ?? this.mcpServers,
    archived: archived ?? this.archived,
  );
}

/// Memory limits the core enforces; the UI checks them first for fast feedback.
const maxMemoryItems = 50;
const maxMemoryItemLength = 1000;

class OutputSummary {
  const OutputSummary({
    required this.id,
    required this.title,
    required this.workType,
    required this.notebookName,
    required this.trigger,
    required this.createdAt,
    this.version = 1,
    this.parentId,
    this.notebookId = '',
  });

  final String notebookId;

  final String id;
  final String title;
  final WorkTypeId workType;
  final String notebookName;
  final String trigger;
  final String createdAt;

  /// 1 for an original output, one more for each revision of it.
  final int version;

  /// The output this version revises.
  final String? parentId;
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

/// Agent mode: the agent called a tool.
class RunToolCall extends RunEvent {
  const RunToolCall({required this.tool, this.args = ''});
  final String tool;
  final String args;
}

/// Agent mode: a tool answered (shortened by the core).
class RunToolResult extends RunEvent {
  const RunToolResult({required this.tool, this.result = ''});
  final String tool;
  final String result;
}

/// Agent mode: the run stopped to wait for approval; it continues in the core
/// once answered.
class RunPaused extends RunEvent {
  const RunPaused({required this.runId, required this.approvals});
  final String runId;
  final List<PendingApproval> approvals;
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
  const CoreEvent({
    required this.kind,
    required this.notebookName,
    this.notebookId = '',
    this.sourceId,
    this.title,
    this.error,
    this.tool,
  });
  final String kind;
  final String notebookName;
  final String notebookId;

  /// The source a research summary event is about.
  final String? sourceId;
  final String? title;
  final String? error;
  final String? tool;

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
    'research.summary.failed' => '$notebookName: a summary failed${error == null ? '' : ' ($error)'}',
    _ => '$notebookName: scheduled run started',
  };
}

/// An agent tool call waiting for the user's answer.
class PendingApproval {
  const PendingApproval({
    required this.runId,
    required this.approvalId,
    required this.tool,
    this.notebookId = '',
    this.notebookName = '',
    this.args = '',
    this.requestedAt = '',
    this.expiresAt = '',
    this.status = 'waiting_approval',
  });
  final String runId;
  final String notebookId;
  final String notebookName;
  final String approvalId;
  final String tool;
  final String args;
  final String requestedAt;
  final String expiresAt;

  /// `waiting_approval`, or `interrupted`: the run stopped (core shut down or crashed) and can be resumed.
  final String status;

  bool get isInterrupted => status == 'interrupted';
}

/// A context slot a template asks the user to fill (never filled data).
class TemplateSlot {
  const TemplateSlot({required this.name, required this.kind, this.description = '', this.required = false});
  final String name;
  final String kind;
  final String description;
  final bool required;
}

/// A reusable notebook blueprint: prompts, slots, memory and an optional schedule.
class TemplateInfo {
  const TemplateInfo({
    required this.id,
    required this.name,
    required this.workType,
    required this.source,
    this.description = '',
    this.version = 1,
    this.parent,
    this.prompts = const [],
    this.slots = const [],
    this.scheduleCron,
  });

  final String id;
  final String name;
  final WorkTypeId workType;
  final String source; // builtin or user
  final String description;
  final int version;
  final String? parent; // id@version this extends
  final List<String> prompts; // prompt names
  final List<TemplateSlot> slots;
  final String? scheduleCron;

  bool get isUser => source == 'user';
}

/// A model provider as configured; the key is a reference, never a value.
class ProviderInfo {
  const ProviderInfo({required this.name, required this.baseUrl, this.keyRef, this.kind = 'openai'});
  final String name;
  final String baseUrl;
  final String? keyRef;

  /// openai (any OpenAI-compatible endpoint), anthropic or gemini.
  final String kind;

  String get kindLabel => switch (kind) {
    'anthropic' => 'Anthropic API',
    'gemini' => 'Gemini API',
    _ => 'OpenAI-compatible',
  };
}

/// A named fallback chain such as "smart" → [provider/model, …].
class AliasInfo {
  const AliasInfo({required this.name, required this.chain, this.fallbackOn = const []});
  final String name;
  final List<String> chain;
  final List<String> fallbackOn;
}

/// Read-only view of the core's settings.
class SettingsInfo {
  const SettingsInfo({
    this.raw = const {},
    this.providers = const [],
    this.aliases = const [],
    this.defaultModel,
    this.thinkingEffort,
    this.maxOutputTokens,
    this.maxTokensPerRun,
    this.maxTokensPerDay,
  });
  final List<ProviderInfo> providers;
  final List<AliasInfo> aliases;
  final String? defaultModel;
  final String? thinkingEffort;
  final int? maxOutputTokens;
  final int? maxTokensPerRun;
  final int? maxTokensPerDay;

  /// The settings JSON as the core sent it (edited through SettingsDraft).
  final Map<String, Object?> raw;
}

/// A notebook's saved prompt with its own model settings (docs/api.md, prompts).
class SavedPrompt {
  const SavedPrompt({required this.name, required this.text, this.model = '', this.temperature, this.maxOutputTokens});

  factory SavedPrompt.fromJson(Map<String, Object?> j) {
    final s = j['settings'] is Map<String, Object?>
        ? j['settings']! as Map<String, Object?>
        : const <String, Object?>{};
    return SavedPrompt(
      name: j['name'] as String? ?? '',
      text: j['text'] as String? ?? '',
      model: s['model'] as String? ?? '',
      temperature: (s['temperature'] as num?)?.toDouble(),
      maxOutputTokens: (s['max_output_tokens'] as num?)?.toInt(),
    );
  }

  final String name;
  final String text;
  final String model;
  final double? temperature;
  final int? maxOutputTokens;

  /// The PUT body: the text and only the settings that are set.
  Map<String, Object?> toBody() {
    final settings = <String, Object?>{
      if (model.isNotEmpty) 'model': model,
      'temperature': ?temperature,
      'max_output_tokens': ?maxOutputTokens,
    };
    return {'text': text, if (settings.isNotEmpty) 'settings': settings};
  }
}

/// One search match (docs/api.md, search).
class SearchHit {
  const SearchHit({required this.kind, required this.notebookId, this.id = '', required this.title, this.snippet = ''});

  factory SearchHit.fromJson(Map<String, Object?> j) => SearchHit(
    kind: j['kind'] as String? ?? '',
    notebookId: j['notebookId'] as String? ?? '',
    id: j['id'] as String? ?? '',
    title: j['title'] as String? ?? '',
    snippet: j['snippet'] as String? ?? '',
  );

  /// notebook, output, prompt, research or note.
  final String kind;
  final String notebookId;
  final String id;
  final String title;
  final String snippet;
}

/// What a past run used (docs/api.md, the run behind an output).
class RunInfo {
  const RunInfo({required this.prompt, this.promptName, this.vars = const {}, this.model = ''});

  factory RunInfo.fromJson(Map<String, Object?> j) => RunInfo(
    prompt: j['prompt'] as String? ?? '',
    promptName: j['promptName'] as String?,
    vars: {
      if (j['vars'] case final Map<String, Object?> v)
        for (final e in v.entries)
          if (e.value case final String s) e.key: s,
    },
    model: j['model'] as String? ?? '',
  );

  final String prompt;
  final String? promptName;
  final Map<String, String> vars;
  final String model;
}

/// One line of a version diff: op is ' ', '+' or '-'.
class DiffLine {
  const DiffLine(this.op, this.text);
  final String op;
  final String text;
}

/// Two versions of an output compared line by line.
class OutputDiffInfo {
  const OutputDiffInfo({required this.from, required this.to, this.lines = const [], this.added = 0, this.removed = 0});

  factory OutputDiffInfo.fromJson(Map<String, Object?> j) {
    final counts = j['counts'] is Map<String, Object?>
        ? j['counts']! as Map<String, Object?>
        : const <String, Object?>{};
    return OutputDiffInfo(
      from: j['from'] as String? ?? '',
      to: j['to'] as String? ?? '',
      lines: [
        for (final l in j['lines'] is List ? j['lines']! as List : const [])
          if (l is Map<String, Object?>)
            DiffLine(String.fromCharCode((l['op'] as num?)?.toInt() ?? 32), l['text'] as String? ?? ''),
      ],
      added: (counts['added'] as num?)?.toInt() ?? 0,
      removed: (counts['removed'] as num?)?.toInt() ?? 0,
    );
  }

  final String from;
  final String to;
  final List<DiffLine> lines;
  final int added;
  final int removed;
}
