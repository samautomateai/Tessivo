part of 'http_core_client.dart';

/// One server-sent event.
class SseEvent {
  const SseEvent(this.event, this.data);
  final String event;
  final String data;
}

/// Parses a text/event-stream. Lines longer than [maxLine] are rejected.
Stream<SseEvent> parseSse(Stream<String> chunks, {int maxLine = 4 * 1024 * 1024}) async* {
  var event = 'message';
  final data = StringBuffer();
  await for (final line in chunks.transform(const LineSplitter())) {
    if (line.length > maxLine) throw const CoreException(502, 'event too large');
    if (line.isEmpty) {
      if (data.isNotEmpty) yield SseEvent(event, data.toString());
      event = 'message';
      data.clear();
    } else if (line.startsWith('event:')) {
      event = line.substring(6).trim();
    } else if (line.startsWith('data:')) {
      if (data.isNotEmpty) data.write('\n');
      data.write(line.substring(5).trimLeft());
    }
  }
  if (data.isNotEmpty) yield SseEvent(event, data.toString());
}
