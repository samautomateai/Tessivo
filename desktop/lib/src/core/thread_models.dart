import 'chat_models.dart';

/// What applying a prompt suggestion changes (docs/api.md, "Output threads").
class PromptTarget {
  const PromptTarget({
    required this.kind,
    required this.current,
    this.promptName,
    this.scheduleId,
    this.schedules = const [],
  });

  final String kind; // saved, schedule or new
  final String current; // the prompt text today
  final String? promptName;
  final String? scheduleId;
  final List<String> schedules;

  /// One line naming everything the change reaches.
  String get reach => switch (kind) {
    'saved' => 'Updates prompt $promptName, used by ${schedules.length} schedule${schedules.length == 1 ? '' : 's'}',
    'schedule' => 'Updates schedule $scheduleId',
    _ => 'Saves a new prompt',
  };
}

/// A proposed change to the notebook from an output's thread.
class Suggestion {
  const Suggestion({
    required this.id,
    required this.kind,
    required this.text,
    required this.status,
    this.reason,
    this.target,
  });

  final String id;
  final String kind; // memory or prompt
  final String text;
  final String status; // open, applied or dismissed
  final String? reason;
  final PromptTarget? target;

  bool get isOpen => status == 'open';
  bool get isPrompt => kind == 'prompt';
}

/// Events while a thread reply streams.
sealed class ThreadEvent {
  const ThreadEvent();
}

class ThreadDelta extends ThreadEvent {
  const ThreadDelta(this.text);
  final String text;
}

/// A suggestion the reply made; it is already stored as open.
class ThreadSuggestion extends ThreadEvent {
  const ThreadSuggestion(this.suggestion);
  final Suggestion suggestion;
}

/// The stored turns once the reply ends.
class ThreadFinished extends ThreadEvent {
  const ThreadFinished(this.user, this.assistant, {this.suggestions = const []});
  final ChatMessage user;
  final ChatMessage assistant;
  final List<Suggestion> suggestions;
}
