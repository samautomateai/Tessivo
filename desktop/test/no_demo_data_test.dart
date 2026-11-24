import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The shipped app must never show made-up data: the sample client lives in
/// test/support/fake_core only.
void main() {
  test('lib has no demo client or sample data', () {
    const banned = [
      'FakeCoreClient',
      'demoOutputs',
      'demoMcpServers',
      'demoLibrary',
      "'Change risk review'",
      "'payments-service'",
      "'Vendor contracts digest'",
    ];
    final offenders = <String>[];
    for (final f in Directory('lib').listSync(recursive: true).whereType<File>()) {
      if (!f.path.endsWith('.dart')) continue;
      final text = f.readAsStringSync();
      for (final b in banned) {
        if (text.contains(b)) offenders.add('${f.path}: $b');
      }
    }
    expect(offenders, isEmpty);
  });
}
