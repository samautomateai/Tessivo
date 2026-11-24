import 'dart:io';

import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/design/icons.dart';

void main() {
  test('every icon has an asset on disk, listed in pubspec', () {
    final pubspec = File('pubspec.yaml').readAsStringSync();
    expect(pubspec, contains('- assets/icons/carbon/'));
    for (final i in TvIcon.values) {
      expect(File(i.asset).existsSync(), isTrue, reason: '${i.name} → ${i.asset}');
      expect(i.asset, startsWith('assets/icons/carbon/'));
    }
    expect(File('assets/icons/carbon/LICENSE').readAsStringSync(), contains('Apache License'));
  });

  test('meanings map to the Carbon icons in the spec', () {
    expect(TvIcon.notebooks.asset, 'assets/icons/carbon/notebook.svg');
    expect(TvIcon.research.asset, 'assets/icons/carbon/microscope.svg');
    expect(TvIcon.approvals.asset, 'assets/icons/carbon/task--approved.svg');
    expect(TvIcon.run.asset, 'assets/icons/carbon/play.svg');
    expect(TvIcon.export.asset, 'assets/icons/carbon/download.svg');
    expect(TvIcon.panelClose.asset, 'assets/icons/carbon/side-panel--close.svg');
    expect(TvIcon.error.asset, 'assets/icons/carbon/error--filled.svg');
  });

  testWidgets('icon renders in the text colour at 16 px with a label', (tester) async {
    await tester.pumpWidget(
      const Directionality(
        textDirection: TextDirection.ltr,
        child: DefaultTextStyle(
          style: TextStyle(color: Color(0xFFFF0000)),
          child: Center(child: TvIconView(TvIcon.run, semanticLabel: 'Run')),
        ),
      ),
    );
    final svg = tester.widget<SvgPicture>(find.byType(SvgPicture));
    expect(svg.colorFilter, const ColorFilter.mode(Color(0xFFFF0000), BlendMode.srcIn));
    expect(tester.getSize(find.byType(SvgPicture)), const Size(16, 16));
    expect(find.bySemanticsLabel('Run'), findsOneWidget);
  });

  testWidgets('size and colour can be set', (tester) async {
    await tester.pumpWidget(
      const Directionality(
        textDirection: TextDirection.ltr,
        child: Center(child: TvIconView(TvIcon.notebooks, size: 20, color: Color(0xFF00FF00))),
      ),
    );
    final svg = tester.widget<SvgPicture>(find.byType(SvgPicture));
    expect(svg.colorFilter, const ColorFilter.mode(Color(0xFF00FF00), BlendMode.srcIn));
    expect(tester.getSize(find.byType(SvgPicture)), const Size(20, 20));
  });
}
