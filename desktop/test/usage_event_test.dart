import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/models.dart';

void main() {
  test('usage events read as plain notices', () {
    const warn = CoreEvent(
      kind: 'usage.warning',
      notebookName: '',
      error: '80% of the daily token limit used: 800 of 1000',
    );
    expect(warn.message, 'Token usage: 80% of the daily token limit used: 800 of 1000');
    const stop = CoreEvent(kind: 'usage.limit', notebookName: '', error: 'daily token limit reached: 1000 of 1000');
    expect(stop.message, 'Token usage: daily token limit reached: 1000 of 1000');
  });
}
