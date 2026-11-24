import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/approval.dart';
import 'package:tessivo_desktop/src/design/components/tool_call.dart';
import 'package:tessivo_desktop/src/screens/chat/chat_side_bar.dart';
import 'package:tessivo_desktop/src/screens/chat/conversation.dart';

import 'support/fake_core/fake_core.dart';

/// A core whose replies first call two tools: one allowed by a rule, one approved.
class _ToolCore extends FakeCoreClient {
  _ToolCore() : super(tick: Duration.zero);

  @override
  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments = const []}) async* {
    yield const ChatToolActivity(kind: 'call', tool: 'run_command', args: '{"cmd":"go test ./pay/..."}');
    yield const ChatToolActivity(kind: 'result', tool: 'run_command', summary: 'ok  payments', by: 'rule');
    yield const ChatToolActivity(kind: 'call', tool: 'mcp__github__get_pr', args: '{"pr":482}');
    yield const ChatToolActivity(kind: 'result', tool: 'mcp__github__get_pr', summary: 'PR #482', by: 'you');
    yield* super.sendChat(chatId, text, attachments: attachments);
  }
}

Future<AppState> pumpChatPage(WidgetTester tester, {Size size = const Size(1600, 1000), FakeCoreClient? core}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(core ?? FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  state.go(Section.chat);
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const Key('chat-c-1')));
  await tester.pumpAndSettle();
  return state;
}

Future<void> send(WidgetTester tester, String text) async {
  await tester.enterText(find.byKey(const Key('chat-input')), text);
  await tester.tap(find.byKey(const Key('chat-send')));
}

void main() {
  test('chats group into Today, Last week and Older', () {
    final now = DateTime(2026, 11, 24, 15);
    expect(chatGroup(DateTime(2026, 11, 24, 1), now), 'Today');
    expect(chatGroup(now.subtract(const Duration(days: 6)), now), 'Last week');
    expect(chatGroup(now.subtract(const Duration(days: 8)), now), 'Older');
  });

  test('tool activity folds into one run per call with its outcome', () {
    final runs = toolRuns(const [
      ChatToolActivity(kind: 'call', tool: 'a', args: '{}'),
      ChatToolActivity(kind: 'result', tool: 'a', summary: 'fine', by: 'rule'),
      ChatToolActivity(kind: 'call', tool: 'b'),
      ChatToolActivity(kind: 'denied', tool: 'b'),
      ChatToolActivity(kind: 'call', tool: 'c'),
    ], const ChatApprovalNeeded(id: '1', tool: 'c'));
    expect(
      [for (final r in runs) (r.tool, r.status)],
      [('a', ToolStatus.done), ('b', ToolStatus.denied), ('c', ToolStatus.waiting)],
    );
    expect(runs.first.by, 'rule');
    expect(
      toolRuns(const [ChatToolActivity(kind: 'result', tool: 'x', summary: 's')], null).single.status,
      ToolStatus.done,
    );
  });

  testWidgets('a reply shows one tool block per call, in order, tagged by who allowed it', (tester) async {
    await pumpChatPage(tester, core: _ToolCore());
    await send(tester, 'Check PR 482');
    await tester.pumpAndSettle();
    final blocks = tester.widgetList<ToolCallBlock>(find.byType(ToolCallBlock)).toList();
    expect([for (final b in blocks) b.tool], ['run_command', 'mcp__github__get_pr']);
    expect(find.descendant(of: find.byType(ToolCallBlock).first, matching: find.text('allow-listed')), findsOneWidget);
    expect(find.descendant(of: find.byType(ToolCallBlock).last, matching: find.text('approved')), findsOneWidget);
  });

  testWidgets('a waiting approval shows in the conversation and as waiting in the panel', (tester) async {
    final state = await pumpChatPage(tester);
    await state.chat.setMcpServers(['github']);
    await tester.pumpAndSettle();
    await send(tester, 'Open issues?');
    await tester.pump();
    await tester.pump();
    expect(find.byType(ApprovalCard), findsOneWidget);
    final panel = find.byKey(const Key('chat-context'));
    expect(find.descendant(of: panel, matching: find.text('waiting')), findsOneWidget);
    await tester.tap(find.text('Allow once'));
    await tester.pumpAndSettle();
    expect(find.byType(ApprovalCard), findsNothing);
    expect(find.descendant(of: panel, matching: find.text('approved')), findsOneWidget);
  });

  testWidgets('the composer shows attach, tools, model, size and Send', (tester) async {
    await pumpChatPage(tester);
    final composer = find.byKey(const Key('composer'));
    for (final key in ['chat-attach', 'chat-tools', 'chat-model', 'chat-send']) {
      expect(
        find.descendant(of: composer, matching: find.byKey(Key(key))),
        findsOneWidget,
        reason: key,
      );
    }
    expect(find.descendant(of: composer, matching: find.text('Tools · 0')), findsOneWidget);
    expect(find.descendant(of: composer, matching: find.textContaining(RegExp(r'^≈ \d+k tokens$'))), findsOneWidget);
    expect(find.descendant(of: composer, matching: find.text('Send')), findsOneWidget);
  });

  testWidgets('at 200 % zoom on a 1366×768 window the chat page does not overflow', (tester) async {
    final state = await pumpChatPage(tester, size: const Size(1366, 768));
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    await state.togglePanel(width: 683);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull, reason: 'panel open at 200 %');
  });
}
