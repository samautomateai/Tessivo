import 'models.dart';

/// Chat limits the core enforces; the UI checks them first for fast feedback.
const maxChatMessageBytes = 64 * 1024;
const maxChatAttachments = 20;
const maxChatTitle = 200;

/// Context attached to a chat message.
class ChatAttachment {
  const ChatAttachment({required this.kind, required this.ref, this.label = '', this.tokens = 0});

  /// path (absolute file or folder), notebook (its context) or output ("notebookId/outputId").
  final String kind;
  final String ref;
  final String label;
  final int tokens;

  String get display {
    if (label.isNotEmpty) return label;
    return ref.split(RegExp(r'[/\\]')).lastWhere((p) => p.isNotEmpty, orElse: () => ref);
  }

  Map<String, Object?> toJson() => {'kind': kind, 'ref': ref, if (label.isNotEmpty) 'label': label};
}

/// One turn in a chat.
class ChatMessage {
  const ChatMessage({
    required this.id,
    required this.role,
    required this.content,
    this.createdAt = '',
    this.model,
    this.stopped = false,
    this.error,
    this.attachments = const [],
  });

  final String id;
  final String role; // user or assistant
  final String content;
  final String createdAt;
  final String? model;
  final bool stopped;
  final String? error;
  final List<ChatAttachment> attachments;

  bool get isUser => role == 'user';
}

/// A chat as listed.
class ChatSummary {
  const ChatSummary({
    required this.id,
    required this.title,
    this.pinned = false,
    this.archived = false,
    this.updatedAt = '',
    this.messageCount = 0,
    this.preview = '',
  });

  final String id;
  final String title;
  final bool pinned;
  final bool archived;
  final String updatedAt;
  final int messageCount;
  final String preview;
}

/// A chat with its settings and messages.
class ChatSession {
  const ChatSession({
    required this.id,
    required this.title,
    this.pinned = false,
    this.archived = false,
    this.model = '',
    this.thinkingEffort = '',
    this.messages = const [],
  });

  final String id;
  final String title;
  final bool pinned;
  final bool archived;
  final String model; // '' uses the app default
  final String thinkingEffort; // '' uses the app default
  final List<ChatMessage> messages;

  ChatSession copyWith({String? title, List<ChatMessage>? messages}) => ChatSession(
    id: id,
    title: title ?? this.title,
    pinned: pinned,
    archived: archived,
    model: model,
    thinkingEffort: thinkingEffort,
    messages: messages ?? this.messages,
  );
}

/// Events while a reply streams.
sealed class ChatEvent {
  const ChatEvent();
}

class ChatDelta extends ChatEvent {
  const ChatDelta(this.text);
  final String text;
}

/// The stored user and assistant turns once the reply ends.
class ChatFinished extends ChatEvent {
  const ChatFinished(this.user, this.assistant);
  final ChatMessage user;
  final ChatMessage assistant;
}

/// What "Make this repeatable" would create.
class RepeatablePlan {
  const RepeatablePlan({
    required this.name,
    required this.workType,
    required this.prompt,
    this.templateId,
    this.templateName,
    this.context = const [],
    this.skipped = const [],
  });

  final String name;
  final WorkTypeId workType;
  final String prompt;
  final String? templateId;
  final String? templateName;
  final List<String> context;
  final List<String> skipped;
}

/// A reply saved to the outputs library.
class ChatOutput {
  const ChatOutput({required this.notebookId, required this.outputId, required this.title});
  final String notebookId;
  final String outputId;
  final String title;
}
