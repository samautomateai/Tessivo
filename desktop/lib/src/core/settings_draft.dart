import 'dart:convert';

/// An editable copy of the core's settings JSON. It edits the raw maps the
/// core sent, so fields the desktop does not show (headers, timeouts, TLS,
/// retries) go back unchanged on save.
class SettingsDraft {
  SettingsDraft.fromJson(Map<String, Object?> raw)
    : providers = _maps(raw['providers']),
      aliases = _maps(raw['aliases']),
      defaults = _map(raw['defaults']),
      limits = _map(raw['limits']),
      desktop = _map(raw['desktop']);

  final List<Map<String, Object?>> providers;
  final List<Map<String, Object?>> aliases;
  final Map<String, Object?> defaults;
  final Map<String, Object?> limits;

  /// The desktop app's own preferences (`desktop` in the config).
  final Map<String, Object?> desktop;

  /// Whether closing the window keeps the app running in the tray.
  bool get closeToTray => desktop['close_to_tray'] == true;

  set closeToTray(bool on) => on ? desktop['close_to_tray'] = true : desktop.remove('close_to_tray');

  static Map<String, Object?> _map(Object? v) =>
      v is Map ? (jsonDecode(jsonEncode(v)) as Map).cast<String, Object?>() : <String, Object?>{};

  static List<Map<String, Object?>> _maps(Object? v) => [
    if (v is List)
      for (final e in v)
        if (e is Map) _map(e),
  ];

  Map<String, Object?>? _provider(String name) => providers.where((p) => p['name'] == name).firstOrNull;

  void setProviderUrl(String name, String url) => _provider(name)?['base_url'] = url;

  void setProviderKeyRef(String name, String ref) {
    final p = _provider(name);
    if (p == null) return;
    if (ref.isEmpty) {
      p.remove('api_key_ref');
    } else {
      p['api_key_ref'] = ref;
    }
  }

  void addProvider(String name, String url, String keyRef, {String kind = 'openai'}) => providers.add({
    'name': name,
    if (kind != 'openai') 'kind': kind,
    'base_url': url,
    if (keyRef.isNotEmpty) 'api_key_ref': keyRef,
  });

  /// Sets an alias's fallback chain from "provider/model" entries.
  void setAliasChain(String name, List<String> chain) {
    final a = aliases.where((a) => a['name'] == name).firstOrNull;
    if (a == null) return;
    a['chain'] = [
      for (final entry in chain)
        if (entry.contains('/'))
          {'provider': entry.substring(0, entry.indexOf('/')), 'model': entry.substring(entry.indexOf('/') + 1)},
    ];
  }

  /// Sets a run default; null or '' removes it (the built-in default applies).
  void setDefault(String key, Object? value) {
    if (value == null || value == '') {
      defaults.remove(key);
    } else {
      defaults[key] = value;
    }
  }

  /// Sets a token limit; null or 0 removes it (no limit).
  void setLimit(String key, int? value) {
    if (value == null || value == 0) {
      limits.remove(key);
    } else {
      limits[key] = value;
    }
  }

  /// The PUT /v1/settings body.
  Map<String, Object?> toBody() => {
    'providers': providers,
    'aliases': aliases,
    'defaults': defaults,
    'limits': limits,
    'desktop': desktop,
  };
}
