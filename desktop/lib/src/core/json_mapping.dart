import 'chat_models.dart';
import 'models.dart';
import 'thread_models.dart';

// Maps the core's JSON (docs/api.md) to UI models. Every field is optional and
// type-checked: responses are treated as untrusted.

ScheduleInfo scheduleFromJson(Map<String, Object?> j) => ScheduleInfo(
  id: j['id'] as String? ?? '',
  cron: j['cron'] as String? ?? '',
  missed: j['missed'] as String? ?? 'skip',
  enabled: j['enabled'] as bool? ?? false,
  prompt: j['prompt'] as String? ?? '',
  nextRun: j['nextRun'] as String?,
  history: jsonList(j['history'])
      .map(
        (h) => ScheduleRun(
          status: h['status'] as String? ?? '',
          startedAt: h['startedAt'] as String? ?? '',
          error: h['error'] as String?,
        ),
      )
      .toList(),
);

CoreEvent eventFromJson(Map<String, Object?> j) => CoreEvent(
  kind: j['kind'] as String? ?? '',
  notebookName: j['notebookName'] as String? ?? '',
  title: j['title'] as String?,
  error: j['error'] as String?,
  tool: j['tool'] as String?,
);

PendingApproval approvalFromJson(Map<String, Object?> j) => PendingApproval(
  runId: j['runId'] as String? ?? '',
  notebookId: j['notebookId'] as String? ?? '',
  notebookName: j['notebookName'] as String? ?? '',
  approvalId: j['approvalId'] as String? ?? '',
  tool: j['tool'] as String? ?? '',
  args: j['args'] as String? ?? '',
  requestedAt: j['requestedAt'] as String? ?? '',
  expiresAt: j['expiresAt'] as String? ?? '',
  status: j['status'] as String? ?? 'waiting_approval',
);

TemplateInfo templateFromJson(Map<String, Object?> j) {
  final schedule = j['schedule'];
  return TemplateInfo(
    id: j['id'] as String? ?? '',
    name: j['name'] as String? ?? '',
    workType: workTypeFrom(j['workType']),
    source: j['source'] as String? ?? 'builtin',
    description: j['description'] as String? ?? '',
    version: (j['version'] as num?)?.toInt() ?? 1,
    parent: j['parent'] as String?,
    prompts: jsonList(j['prompts']).map((p) => p['name'] as String? ?? '').toList(),
    slots: jsonList(j['slots'])
        .map(
          (s) => TemplateSlot(
            name: s['name'] as String? ?? '',
            kind: s['kind'] as String? ?? 'any',
            description: s['description'] as String? ?? '',
            required: s['required'] as bool? ?? false,
          ),
        )
        .toList(),
    scheduleCron: schedule is Map<String, Object?> ? schedule['cron'] as String? : null,
  );
}

List<Map<String, Object?>> jsonList(Object? v) =>
    v is List ? v.whereType<Map<String, Object?>>().toList() : const <Map<String, Object?>>[];

WorkTypeId workTypeFrom(Object? v) =>
    WorkTypeId.values.firstWhere((w) => w.name == v, orElse: () => WorkTypeId.summarize);

Notebook notebookFromJson(Map<String, Object?> j) => Notebook(
  id: j['id'] as String? ?? '',
  name: j['name'] as String? ?? '',
  workType: workTypeFrom(j['workType']),
  context: jsonList(j['context']).map(contextFromJson).toList(),
  memory: _strings(j['memory']),
);

ContextItem contextFromJson(Map<String, Object?> j) {
  final label = j['label'] as String? ?? '';
  final dot = label.lastIndexOf('.');
  final kind = (j['inPlace'] as bool? ?? false)
      ? 'Folder'
      : (dot > 0 ? label.substring(dot + 1).toUpperCase() : 'File');
  return ContextItem(
    id: j['id'] as String? ?? '',
    label: label,
    kind: kind,
    enabled: j['enabled'] as bool? ?? false,
    estimatedTokens: (j['estimatedTokens'] as num?)?.toInt() ?? 0,
  );
}

OutputSummary outputFromJson(Map<String, Object?> j) => OutputSummary(
  id: j['id'] as String? ?? '',
  title: j['title'] as String? ?? '',
  workType: workTypeFrom(j['workType']),
  notebookName: j['notebookName'] as String? ?? '',
  notebookId: j['notebookId'] as String? ?? '',
  trigger: 'Manual',
  createdAt: j['createdAt'] as String? ?? '',
  version: (j['version'] as num?)?.toInt() ?? 1,
  parentId: j['parentId'] as String?,
);

RunOutcome outcomeFromJson(Map<String, Object?> j) {
  final attempts = jsonList(j['attempts']);
  final fell = attempts.where((a) => (a['trigger'] as String? ?? '').isNotEmpty).toList();
  return RunOutcome(
    valid: j['valid'] as bool? ?? false,
    modelUsed: j['modelUsed'] as String? ?? '',
    totalTokens: (j['totalTokens'] as num?)?.toInt() ?? 0,
    markdown: j['markdown'] as String? ?? '',
    fallbackNote: fell.isEmpty ? null : 'fell back after ${fell.map((a) => a['trigger']).join(', ')}',
  );
}

List<String> _strings(Object? v) => v is List ? v.whereType<String>().toList() : const [];

int? _int(Object? v) => v is num ? v.toInt() : null;

/// A JSON object, or an empty one when [v] is anything else.
Map<String, Object?> jsonObject(Object? v) => v is Map<String, Object?> ? v : const {};

OutputDetail outputDetailFromJson(Map<String, Object?> j) => OutputDetail(
  id: j['id'] as String? ?? '',
  title: j['title'] as String? ?? '',
  markdown: j['markdown'] as String? ?? '',
);

/// Reads only the fields the UI shows: provider headers are skipped entirely.
SettingsInfo settingsFromJson(Map<String, Object?> j) {
  final defaults = jsonObject(j['defaults']);
  final limits = jsonObject(j['limits']);
  return SettingsInfo(
    providers: jsonList(j['providers'])
        .map(
          (p) => ProviderInfo(
            name: p['name'] as String? ?? '',
            baseUrl: p['base_url'] as String? ?? '',
            keyRef: p['api_key_ref'] as String?,
          ),
        )
        .toList(),
    aliases: jsonList(j['aliases'])
        .map(
          (a) => AliasInfo(
            name: a['name'] as String? ?? '',
            chain: jsonList(a['chain']).map((m) => '${m['provider'] ?? '?'}/${m['model'] ?? '?'}').toList(),
            fallbackOn: _strings(a['fallback_on']),
          ),
        )
        .toList(),
    defaultModel: defaults['model'] as String?,
    thinkingEffort: defaults['thinking_effort'] as String?,
    maxOutputTokens: _int(defaults['max_output_tokens']),
    maxTokensPerRun: _int(limits['max_tokens_per_run']),
    maxTokensPerDay: _int(limits['max_tokens_per_day']),
  );
}

/// `{"models": {"provider": ["model", …]}}` → provider → models, sorted.
Map<String, List<String>> modelsFromJson(Map<String, Object?> j) {
  final m = j['models'];
  if (m is! Map<String, Object?>) return const {};
  return {
    for (final e in m.entries) e.key: [..._strings(e.value)]..sort(),
  };
}

ChatAttachment chatAttachmentFromJson(Map<String, Object?> j) => ChatAttachment(
  kind: j['kind'] as String? ?? '',
  ref: j['ref'] as String? ?? '',
  label: j['label'] as String? ?? '',
  tokens: _int(j['tokens']) ?? 0,
);

ChatMessage chatMessageFromJson(Map<String, Object?> j) => ChatMessage(
  id: j['id'] as String? ?? '',
  role: switch (j['role']) {
    'user' => 'user',
    'note' => 'note',
    _ => 'assistant',
  },
  content: j['content'] as String? ?? '',
  createdAt: j['createdAt'] as String? ?? '',
  model: j['model'] as String?,
  stopped: j['stopped'] as bool? ?? false,
  error: j['error'] as String?,
  attachments: jsonList(j['attachments']).map(chatAttachmentFromJson).toList(),
);

ChatSummary chatSummaryFromJson(Map<String, Object?> j) => ChatSummary(
  id: j['id'] as String? ?? '',
  title: j['title'] as String? ?? '',
  pinned: j['pinned'] as bool? ?? false,
  archived: j['archived'] as bool? ?? false,
  updatedAt: j['updatedAt'] as String? ?? '',
  messageCount: _int(j['messageCount']) ?? 0,
  preview: j['preview'] as String? ?? '',
);

ChatSession chatSessionFromJson(Map<String, Object?> j) => ChatSession(
  id: j['id'] as String? ?? '',
  title: j['title'] as String? ?? '',
  pinned: j['pinned'] as bool? ?? false,
  archived: j['archived'] as bool? ?? false,
  model: j['model'] as String? ?? '',
  thinkingEffort: j['thinkingEffort'] as String? ?? '',
  messages: jsonList(j['messages']).map(chatMessageFromJson).toList(),
);

RepeatablePlan repeatablePlanFromJson(Map<String, Object?> j) => RepeatablePlan(
  name: j['name'] as String? ?? '',
  workType: workTypeFrom(j['workType']),
  prompt: j['prompt'] as String? ?? '',
  templateId: j['templateId'] as String?,
  templateName: j['templateName'] as String?,
  context: _strings(j['context']),
  skipped: _strings(j['skipped']),
);

ChatOutput chatOutputFromJson(Map<String, Object?> j) => ChatOutput(
  notebookId: j['notebookId'] as String? ?? '',
  outputId: j['outputId'] as String? ?? '',
  title: j['title'] as String? ?? '',
);

/// A JSON array of strings; other entries are dropped.
List<String> jsonStrings(Object? v) => _strings(v);

PromptTarget promptTargetFromJson(Map<String, Object?> j) => PromptTarget(
  kind: j['kind'] as String? ?? 'new',
  current: j['current'] as String? ?? '',
  promptName: j['promptName'] as String?,
  scheduleId: j['scheduleId'] as String?,
  schedules: _strings(j['schedules']),
);

Suggestion suggestionFromJson(Map<String, Object?> j) {
  final target = j['target'];
  return Suggestion(
    id: j['id'] as String? ?? '',
    kind: j['kind'] == 'prompt' ? 'prompt' : 'memory',
    text: j['text'] as String? ?? '',
    status: j['status'] as String? ?? 'open',
    reason: j['reason'] as String?,
    target: target is Map<String, Object?> ? promptTargetFromJson(target) : null,
  );
}
