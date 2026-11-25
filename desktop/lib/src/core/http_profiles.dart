part of 'http_core_client.dart';

/// Settings profiles and work type suggestions (docs/api.md).
mixin _HttpProfiles on _HttpChat {
  Future<ProfilesInfo> profiles() async => ProfilesInfo.fromJson(await _json('GET', '/v1/profiles'));

  Future<ProfilesInfo> saveProfile(String name) async =>
      ProfilesInfo.fromJson(await _json('POST', '/v1/profiles', body: {'name': name}));

  Future<ProfilesInfo> useProfile(String name) async =>
      ProfilesInfo.fromJson(await _json('POST', '/v1/profiles/${Uri.encodeComponent(name)}/use'));

  Future<ProfilesInfo> deleteProfile(String name) async =>
      ProfilesInfo.fromJson(await _json('DELETE', '/v1/profiles/${Uri.encodeComponent(name)}'));

  Future<WorkTypeSuggestion> workTypeSuggestion(String notebookId) async => WorkTypeSuggestion.fromJson(
    await _json('GET', '/v1/notebooks/${Uri.encodeComponent(notebookId)}/worktype-suggestion'),
  );
}
