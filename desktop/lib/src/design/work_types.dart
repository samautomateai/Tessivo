import '../core/models.dart';

/// The name of a work type as the UI shows it.
String workTypeLabel(WorkTypeId id) => switch (id) {
  WorkTypeId.analyze => 'Analyze',
  WorkTypeId.summarize => 'Summarize',
  WorkTypeId.document => 'Document',
  WorkTypeId.report => 'Report',
};
