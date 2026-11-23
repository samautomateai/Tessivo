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

/// Throws a [FormatException] unless [url] is an http(s) address without
/// credentials, the web pages the core will fetch.
void checkWebUrl(String url) {
  final u = Uri.tryParse(url.trim());
  if (u == null || !(u.isScheme('http') || u.isScheme('https')) || u.host.isEmpty || u.userInfo.isNotEmpty) {
    throw const FormatException('Enter an http:// or https:// address without a user name or password');
  }
  if (url.length > 2048) throw const FormatException('The address is longer than 2048 characters');
}
