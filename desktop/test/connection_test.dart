import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/connection.dart';
import 'package:tessivo_desktop/src/core/supervisor.dart';

import 'app_test.dart' show pumpApp;
import 'support/fake_core/fake_core.dart';

void main() {
  testWidgets('shows starting, then the app', (tester) async {
    final done = Completer<CoreClient>();
    await tester.pumpWidget(
      ConnectionGate(
        connect: () => done.future,
        builder: (_) => const MaterialApp(home: Text('the app')),
      ),
    );
    expect(find.text('Starting the Tessivo core…'), findsOneWidget);
    done.complete(FakeCoreClient(tick: Duration.zero));
    await tester.pumpAndSettle();
    expect(find.text('the app'), findsOneWidget);
  });

  testWidgets('shows why it failed with retry and details', (tester) async {
    var calls = 0;
    Future<CoreClient> connect() async {
      calls++;
      throw const CoreStartException('tess serve exited: bad config');
    }

    await tester.pumpWidget(
      ConnectionGate(
        connect: connect,
        builder: (_) => const MaterialApp(home: Text('the app')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text("Can't reach the Tessivo core"), findsOneWidget);
    expect(find.textContaining('tess serve exited: bad config'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
    expect(find.text('Copy details'), findsOneWidget);
    expect(find.textContaining('tess serve'), findsWidgets);
    await tester.tap(find.text('Retry'));
    await tester.pumpAndSettle();
    expect(calls, 2);
  });

  testWidgets('reconnects after the core goes away', (tester) async {
    final state = await pumpApp(tester);
    final core = state.core as FakeCoreClient;
    state.listenForEvents();
    core.offline = true;
    core.failEvents();
    await tester.pump();
    await tester.pump();
    expect(state.connectionLost, isTrue);
    expect(find.text('Reconnecting to the Tessivo core…'), findsOneWidget);
    core.offline = false;
    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
    expect(state.connectionLost, isFalse);
    expect(find.text('Reconnecting to the Tessivo core…'), findsNothing);
  });

  test('reconnect waits 1, 2, 4, 8, 15 then 15 seconds', () {
    expect([for (var i = 0; i < 7; i++) reconnectDelay(i).inSeconds], [1, 2, 4, 8, 15, 15, 15]);
  });

  test('TESSIVO_TESS overrides the bundled tess', () {
    final dir = Directory.systemTemp.createTempSync('tv-');
    addTearDown(() => dir.deleteSync(recursive: true));
    final tess = File('${dir.path}/tess-dev')..writeAsStringSync('x');
    expect(findTessExecutable(resolvedExecutable: '${dir.path}/app', isWindows: false, override: tess.path), tess.path);
    expect(
      findTessExecutable(resolvedExecutable: '${dir.path}/app', isWindows: false, override: '${dir.path}/missing'),
      'tess',
    );
  });

  test('AppState starts connected', () {
    expect(AppState(FakeCoreClient(tick: Duration.zero)).connectionLost, isFalse);
  });
}
