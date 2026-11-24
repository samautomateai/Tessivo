import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Screens and widgets take colours and type from lib/src/design only. Files
/// still on the old styling are listed here; the list may only shrink, and is
/// empty once every screen is on the design system.
const _notMovedYet = {'lib/src/widgets/output_edit_form.dart'};

final _raw = RegExp(r'Color\(0x|fontSize:|(?<![A-Za-z])Colors\.');

void main() {
  test('screens and widgets use design tokens, not raw colours or font sizes', () {
    final offenders = <String>{};
    for (final dir in ['lib/src/screens', 'lib/src/widgets', 'lib/src/shell']) {
      final d = Directory(dir);
      if (!d.existsSync()) continue;
      for (final f in d.listSync(recursive: true).whereType<File>().where((f) => f.path.endsWith('.dart'))) {
        if (_raw.hasMatch(f.readAsStringSync())) offenders.add(f.path.replaceAll(r'\', '/'));
      }
    }
    expect(offenders.difference(_notMovedYet), isEmpty, reason: 'new files must use lib/src/design');
    expect(_notMovedYet.difference(offenders), isEmpty, reason: 'moved files: remove them from _notMovedYet');
  });
}
