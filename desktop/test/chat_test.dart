import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/chat_state.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/widgets/chat_widgets.dart';

import 'support/fake_core/fake_core.dart';

Future<AppState> pumpChat(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  await tester.tap(find.text('Chat'));
  await tester.pumpAndSettle();
  return state;
}

Future<void> answer(WidgetTester tester, String value) async {
  await tester.enterText(find.byKey(const Key('text-prompt')), value);
  await tester.tap(find.byKey(const Key('text-prompt-ok')));
  await tester.pumpAndSettle();
}

void main() {
  group('chat state', () {
    test('loads, sends and reloads the stored turns', () async {
      final chat = ChatState(FakeCoreClient(tick: Duration.zero));
      await chat.load();
      expect(chat.sessions.first.pinned, isTrue);
      expect(chat.memory, isNotEmpty);
      expect(await chat.send('Summarize the vendor contracts'), isTrue);
      expect(chat.open!.title, 'Summarize the vendor contracts');
      expect(chat.open!.messages.last.content, startsWith('Here is a sample reply'));
      expect(chat.sending, isFalse);
      expect(await chat.send('  '), isFalse);
      expect(chat.error, 'Type a message first');
      chat.dispose();
    });

    test('stop keeps the partial reply and marks it stopped', () async {
      final chat = ChatState(FakeCoreClient(tick: const Duration(milliseconds: 5)));
      await chat.newChat();
      final sending = chat.send('Explain the architecture');
      while (chat.streaming.isEmpty) {
        await Future<void>.delayed(const Duration(milliseconds: 2));
      }
      await chat.stop();
      expect(await sending, isTrue);
      final reply = chat.open!.messages.last;
      expect(reply.stopped, isTrue);
      expect(reply.content, isNotEmpty);
      expect(chat.sending, isFalse);
      await chat.stop(); // nothing to stop: no-op
      chat.dispose();
    });

    test('attachments, filters, updates, plan and errors', () async {
      final chat = ChatState(FakeCoreClient(tick: Duration.zero));
      await chat.load();
      await chat.select('c-1');
      const a = ChatAttachment(kind: 'path', ref: '/srv/repo');
      chat
        ..attach(a)
        ..attach(a);
      expect(chat.pending, hasLength(1));
      chat.detach(a);
      expect(chat.pending, isEmpty);
      for (var i = 0; i <= maxChatAttachments; i++) {
        chat.attach(ChatAttachment(kind: 'path', ref: '/f$i'));
      }
      expect(chat.error, contains('attachments'));
      chat.clearError();

      await chat.search('vendor');
      expect(chat.sessions.single.id, 'c-2');
      await chat.search('');
      await chat.setArchived('c-2', true);
      expect(chat.sessions.map((s) => s.id), isNot(contains('c-2')));
      await chat.toggleArchived();
      expect(chat.sessions.single.id, 'c-2');
      await chat.toggleArchived();

      await chat.rename('c-1', 'two\nlines');
      expect(chat.error, contains('one line'));
      await chat.setModel('smart');
      await chat.setThinking('high');
      expect(chat.open!.model, 'smart');
      expect(chat.open!.thinkingEffort, 'high');

      final plan = await chat.plan();
      expect(plan!.prompt, contains('rotate'));
      final out = await chat.saveReply('m2', WorkTypeId.report);
      expect(out!.notebookId, 'chat-report');
      expect(await chat.saveReply('m1', WorkTypeId.report), isNull);
      expect(chat.error, contains('Only non-empty replies'));

      await chat.setMemory(['a', 'b']);
      expect(chat.memory, ['a', 'b']);
      await chat.delete('c-1');
      expect(chat.open, isNull);
      await chat.select('c-1');
      expect(chat.error, contains('unknown chat'));
      chat.dispose();
    });
  });

  testWidgets('opens a chat with Markdown and sends a message', (tester) async {
    await pumpChat(tester);
    expect(find.byKey(const Key('chat-start')), findsOneWidget);
    await tester.tap(find.byKey(const Key('chat-c-1')));
    await tester.pumpAndSettle();
    expect(find.text('Short answer'), findsOneWidget);
    expect(find.byType(Table), findsOneWidget);

    await tester.enterText(find.byKey(const Key('chat-input')), 'And for API keys?');
    await tester.tap(find.byKey(const Key('chat-send')));
    await tester.pumpAndSettle();
    expect(find.text('And for API keys?'), findsOneWidget);
    final reply = find.descendant(
      of: find.byType(MessageList),
      matching: find.textContaining('sample reply from the demo core'),
    );
    expect(reply, findsOneWidget);
  });

  testWidgets('new chat and attachments', (tester) async {
    final state = await pumpChat(tester);
    await tester.tap(find.byKey(const Key('chat-new')));
    await tester.pumpAndSettle();
    expect(find.text('Say hello, or attach files and ask about them.'), findsOneWidget);

    await tester.tap(find.byKey(const Key('chat-attach')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('File or folder…'));
    await tester.pumpAndSettle();
    await answer(tester, 'relative/path');
    expect(find.textContaining('Use an absolute path'), findsOneWidget);
    tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger)).removeCurrentSnackBar();
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('chat-attach')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Notebook: Change risk review'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('pending-nb-1')), findsOneWidget);
    tester.widget<InputChip>(find.byKey(const Key('pending-nb-1'))).onDeleted!();
    await tester.pumpAndSettle();
    expect(state.chat.pending, isEmpty);

    await tester.tap(find.byKey(const Key('chat-attach')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-attach-url')));
    await tester.pumpAndSettle();
    await answer(tester, 'ftp://files.example.com/x');
    expect(find.textContaining('Enter an http:// or https:// address'), findsOneWidget);
    tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger)).removeCurrentSnackBar();
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-attach')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-attach-url')));
    await tester.pumpAndSettle();
    await answer(tester, 'https://status.example.com/today/');
    expect(find.text('status.example.com/today'), findsOneWidget);
    expect(state.chat.pending.single.kind, 'url');
  });

  testWidgets('session menu pins, renames, archives and deletes', (tester) async {
    final state = await pumpChat(tester);
    Future<void> menu(String id, String item) async {
      await tester.tap(find.byKey(Key('chat-menu-$id')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(item).last);
      await tester.pumpAndSettle();
    }

    await menu('c-2', 'Pin');
    expect(state.chat.sessions.where((s) => s.pinned), hasLength(2));
    await menu('c-2', 'Rename');
    await answer(tester, 'Vendor spend Q3');
    expect(find.text('Vendor spend Q3'), findsOneWidget);
    await menu('c-2', 'Archive');
    expect(find.byKey(const Key('chat-c-2')), findsNothing);
    await tester.tap(find.byKey(const Key('chat-archived')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('chat-c-2')), findsOneWidget);
    await menu('c-2', 'Delete');
    expect(find.text('Nothing archived.'), findsOneWidget);

    await tester.tap(find.byKey(const Key('chat-archived')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('chat-search')), 'zzz');
    await tester.pumpAndSettle();
    expect(find.text('No chats match.'), findsOneWidget);
  });

  testWidgets('saves a reply and makes a chat repeatable', (tester) async {
    final state = await pumpChat(tester);
    await tester.tap(find.byKey(const Key('chat-c-1')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('save-m2')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Report'));
    await tester.tap(find.byKey(const Key('save-confirm')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('saved-output')), findsOneWidget);

    await tester.tap(find.byKey(const Key('chat-repeat')));
    await tester.pumpAndSettle();
    expect(find.textContaining('How often should we rotate'), findsWidgets);
    await tester.enterText(find.byKey(const Key('repeat-name')), 'Key rotation');
    await tester.tap(find.byKey(const Key('repeat-create')));
    await tester.pumpAndSettle();
    expect(state.section, Section.notebooks);
    expect(state.selected!.name, 'Key rotation');
  });

  testWidgets('chat memory dialog adds and removes facts', (tester) async {
    final state = await pumpChat(tester);
    await tester.tap(find.byKey(const Key('chat-memory')));
    await tester.pumpAndSettle();
    expect(find.text('I lead the payments platform team'), findsOneWidget);
    await tester.tap(find.byKey(const Key('chat-memory-add')));
    await tester.pumpAndSettle();
    await answer(tester, 'Prefer tables');
    expect(state.chat.memory, hasLength(2));
    await tester.tap(find.byKey(const Key('chat-memory-remove-0')));
    await tester.pumpAndSettle();
    expect(state.chat.memory, ['Prefer tables']);
    await tester.tap(find.text('Done'));
    await tester.pumpAndSettle();
    expect(find.text('Memory (1)'), findsOneWidget);
  });

  testWidgets('chat errors show in a banner', (tester) async {
    tester.view.physicalSize = const Size(1600, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final state = AppState(_FailingChat());
    await tester.pumpWidget(TessivoApp(state: state));
    await state.load();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Chat'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-c-1')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('chat-input')), 'hi');
    await tester.tap(find.byKey(const Key('chat-send')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('chat-error')), findsOneWidget);
    expect(find.textContaining('model down'), findsOneWidget);
    await tester.tap(find.text('Dismiss'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('chat-error')), findsNothing);
  });
}

class _FailingChat extends FakeCoreClient {
  _FailingChat() : super(tick: Duration.zero);

  @override
  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments = const []}) =>
      Stream.error(const CoreException(502, 'model down'));

  @override
  Future<SettingsInfo> getSettings() => Future.error(StateError('offline'));
}
