/// One output: its Markdown and the structured result it was rendered from.
class OutputDetail {
  const OutputDetail({required this.id, required this.title, required this.markdown, this.data = const {}});
  final String id;
  final String title;
  final String markdown;

  /// The structured result (work-type fields); what the edit form changes.
  final Map<String, Object?> data;
}
