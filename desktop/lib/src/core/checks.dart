import 'chat_models.dart';
import 'models.dart';

/// Context paths must be absolute (the core resolves nothing relative to itself).
void checkContextPath(String path) {
  final absolute = path.startsWith('/') || RegExp(r'^[A-Za-z]:[\\/]').hasMatch(path) || path.startsWith(r'\\');
  if (path.trim() != path || path.isEmpty || !absolute) {
    throw const FormatException('Use an absolute path, for example /Users/me/project');
  }
}

/// Mirrors the core's memory limits for fast feedback.
void checkMemory(List<String> memory) {
  if (memory.length > maxMemoryItems) {
    throw const FormatException('Keep memory to $maxMemoryItems items');
  }
  if (memory.any((m) => m.trim().isEmpty || m.length > maxMemoryItemLength)) {
    throw const FormatException('Each memory item must be 1–$maxMemoryItemLength characters');
  }
}

/// Mirrors the core's message checks for fast feedback.
void checkChatMessage(String text, List<ChatAttachment> attachments) {
  if (text.trim().isEmpty) throw const FormatException('Type a message first');
  if (text.length > maxChatMessageBytes) {
    throw const FormatException('Messages are limited to ${maxChatMessageBytes ~/ 1024} KiB');
  }
  if (attachments.length > maxChatAttachments) {
    throw const FormatException('At most $maxChatAttachments attachments per message');
  }
}
