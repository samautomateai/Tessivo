import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/design/scale_probe.dart';

void main() {
  test('describeScale formats display ratio and text scale', () {
    const mq = MediaQueryData(devicePixelRatio: 1.5, textScaler: TextScaler.linear(1.25));
    expect(describeScale(mq), 'display 1.50 × text 1.25');
  });
}
