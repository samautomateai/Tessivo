import 'package:tessivo_desktop/src/core/errors.dart';
import 'package:tessivo_desktop/src/core/json_mapping.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'fake_settings.dart';

/// Settings profiles and work type suggestions for the demo core.
mixin DemoProfiles on DemoSettings {
  final List<ProfileSummary> demoProfiles = [];
  String demoActiveProfile = '';

  /// Suggestions per notebook ID, set by tests.
  final Map<String, WorkTypeSuggestion> demoSuggestions = {};

  ProfilesInfo get _info => ProfilesInfo(active: demoActiveProfile, profiles: List.of(demoProfiles));

  /// The settings with the active profile, as the core reports it.
  @override
  Future<SettingsInfo> getSettings() async =>
      settingsFromJson({...(await super.getSettings()).raw, 'profile': demoActiveProfile});

  Future<ProfilesInfo> profiles() async => _info;

  Future<ProfilesInfo> saveProfile(String name) async {
    if (!RegExp(r'^[a-z0-9][a-z0-9_-]{0,31}$').hasMatch(name)) {
      throw CoreException(400, 'profile name "$name": use 1-32 lower-case letters, digits, - or _');
    }
    demoProfiles
      ..removeWhere((p) => p.name == name)
      ..add(ProfileSummary(name: name, providers: 1, defaultModel: 'smart'));
    demoActiveProfile = name;
    return _info;
  }

  Future<ProfilesInfo> useProfile(String name) async {
    if (!demoProfiles.any((p) => p.name == name)) throw CoreException(404, 'profile "$name" not found');
    demoActiveProfile = name;
    return _info;
  }

  Future<ProfilesInfo> deleteProfile(String name) async {
    if (!demoProfiles.any((p) => p.name == name)) throw CoreException(404, 'profile "$name" not found');
    demoProfiles.removeWhere((p) => p.name == name);
    if (demoActiveProfile == name) demoActiveProfile = '';
    return _info;
  }

  Future<WorkTypeSuggestion> workTypeSuggestion(String notebookId) async =>
      demoSuggestions[notebookId] ?? const WorkTypeSuggestion();
}
