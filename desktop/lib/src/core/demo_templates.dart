import 'models.dart';

/// Sample of the core's built-in templates for the demo client.
const demoTemplates = <TemplateInfo>[
  TemplateInfo(
    id: 'change-risk-review',
    name: 'Change risk review',
    workType: WorkTypeId.analyze,
    source: 'builtin',
    description: 'Score the risk of a change against the code base and past incidents.',
    prompts: ['review', 'rollback-plan'],
    slots: [
      TemplateSlot(name: 'Code base', kind: 'code base', required: true),
      TemplateSlot(name: 'Diff or PR', kind: 'diff', required: true),
      TemplateSlot(name: 'Incident history', kind: 'spreadsheet'),
    ],
  ),
  TemplateInfo(
    id: 'spreadsheet-insights',
    name: 'Spreadsheet insights',
    workType: WorkTypeId.analyze,
    source: 'builtin',
    description: 'Find trends, outliers and questions worth asking in tabular data.',
    prompts: ['insights'],
    slots: [TemplateSlot(name: 'Spreadsheet', kind: 'spreadsheet', required: true)],
  ),
  TemplateInfo(
    id: 'code-base-overview',
    name: 'Code base overview',
    workType: WorkTypeId.document,
    source: 'builtin',
    description: 'Architecture, modules and how a request flows through them.',
    prompts: ['overview'],
    slots: [TemplateSlot(name: 'Code base', kind: 'code base', required: true)],
  ),
  TemplateInfo(
    id: 'meeting-notes-to-actions',
    name: 'Meeting notes to actions',
    workType: WorkTypeId.summarize,
    source: 'builtin',
    description: 'Decisions, owners and open questions from meeting notes.',
    prompts: ['actions'],
    slots: [TemplateSlot(name: 'Notes', kind: 'documents', required: true)],
  ),
  TemplateInfo(
    id: 'contract-digest',
    name: 'Contract digest',
    workType: WorkTypeId.summarize,
    source: 'builtin',
    description: 'Terms, renewal dates, obligations and unusual clauses.',
    prompts: ['digest'],
    slots: [TemplateSlot(name: 'Contracts', kind: 'documents', required: true)],
  ),
  TemplateInfo(
    id: 'weekly-ops-report',
    name: 'Weekly ops report',
    workType: WorkTypeId.report,
    source: 'builtin',
    description: 'What happened this week, what is at risk and what is next.',
    prompts: ['weekly'],
    slots: [
      TemplateSlot(name: 'Tickets', kind: 'spreadsheet', required: true),
      TemplateSlot(name: 'Previous report', kind: 'documents'),
    ],
    scheduleCron: '0 8 * * MON',
  ),
];

/// Mirrors the core's slug rule: lowercase letters, digits and '-', at most 63 chars.
String templateIdFor(String name) {
  final slug = name
      .toLowerCase()
      .replaceAll(RegExp('[^a-z0-9]+'), '-')
      .replaceAll(RegExp('^-+|-+\$'), '');
  return slug.length > 63 ? slug.substring(0, 63).replaceAll(RegExp(r'-+$'), '') : slug;
}
