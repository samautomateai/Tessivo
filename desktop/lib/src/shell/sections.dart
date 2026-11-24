import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../screens/chat/chat_context_panel.dart';
import '../screens/chat/chat_page.dart';
import '../screens/chat/chat_side_bar.dart';
import '../screens/library_screen.dart';
import '../screens/notebook/notebook_page.dart';
import '../screens/notebook/notebook_side_bar.dart';
import '../screens/notebook/thread_panel.dart';
import '../screens/research_screen.dart';
import '../screens/schedules_screen.dart';
import '../screens/settings/settings_page.dart';
import '../screens/settings/settings_side_bar.dart';
import '../screens/templates_screen.dart';
import 'side_bar.dart';

/// The page each section shows in the main area.
Widget pageFor(AppState state) => switch (state.section) {
  Section.chat => ChatPage(state: state),
  Section.notebooks => NotebookPage(state: state),
  Section.research => ResearchScreen(state: state),
  Section.templates => TemplatesScreen(state: state),
  Section.schedules => SchedulesScreen(state: state),
  Section.library => LibraryScreen(core: state.core),
  Section.settings => SettingsPage(state: state, onChanged: () => unawaited(state.checkSetup())),
};

/// The side bar of a section, or null when it has none (yet).
Widget? sideBarFor(AppState state) => switch (state.section) {
  Section.notebooks => NotebookSideBar(state: state),
  Section.chat => ChatSideBar(state: state),
  Section.settings => SettingsSideBar(state: state),
  Section.research => ResearchSideBar(state: state),
  _ => null,
};

/// The secondary panel of a section (thread, chat context), or null.
Widget? panelFor(AppState state) => switch ((state.section, state.thread)) {
  (Section.chat, _) => ChatContextPanel(state: state),
  (Section.notebooks, final thread?) when state.selected != null => ThreadPanel(
    key: ValueKey(thread),
    state: state,
    thread: thread,
  ),
  _ => null,
};
