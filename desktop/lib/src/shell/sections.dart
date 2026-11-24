import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../screens/chat_screen.dart';
import '../screens/library_screen.dart';
import '../screens/research_screen.dart';
import '../screens/schedules_screen.dart';
import '../screens/settings_screen.dart';
import '../screens/templates_screen.dart';
import '../screens/workspace_screen.dart';
import 'side_bar.dart';

/// The page each section shows in the main area.
Widget pageFor(AppState state) => switch (state.section) {
  Section.chat => ChatScreen(state: state),
  Section.notebooks => WorkspaceScreen(state: state),
  Section.research => ResearchScreen(state: state),
  Section.templates => TemplatesScreen(state: state),
  Section.schedules => SchedulesScreen(state: state),
  Section.library => LibraryScreen(core: state.core),
  Section.settings => SettingsScreen(core: state.core, state: state, onChanged: () => unawaited(state.checkSetup())),
};

/// The side bar of a section, or null when it has none (yet).
Widget? sideBarFor(AppState state) => switch (state.section) {
  Section.notebooks => NotebooksSideBar(state: state),
  Section.research => ResearchSideBar(state: state),
  _ => null,
};

/// The secondary panel of a section (thread, chat context), or null.
Widget? panelFor(AppState state) => null;
