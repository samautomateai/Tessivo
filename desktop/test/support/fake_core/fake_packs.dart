import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'fake_mcp.dart';

/// Team packs in the fake core: adding any source adds the "team" pack with
/// a required template, a prompt and a required MCP server that is not trusted.
mixin DemoPacks on DemoMcp {
  /// The fake core's template list, which pack templates join.
  List<TemplateInfo> get templatesForPacks;

  final List<PackInfo> _packs = [];

  /// How many times the packs were refreshed.
  int packRefreshes = 0;

  static const _team = PackInfo(
    id: 'team',
    name: 'Team pack',
    source: 'https://git.example.com/team/pack.git',
    kind: 'git',
    publisher: 'Platform team',
    fetchedAt: '2026-11-25T08:00:00Z',
    templates: 1,
    prompts: 1,
    servers: 1,
    problems: ['templates/broken.json: want 1-20 prompts'],
    readme: '# Team pack\n\nUse the **risk review** for every release.',
  );

  Future<List<PackInfo>> listPacks() async => List.unmodifiable(_packs);

  Future<PackInfo> addPack(String source, {String branch = ''}) async {
    if (source.startsWith('file:')) throw const CoreException(400, 'use an https:// or git@ URL, a folder or a .zip');
    if (_packs.any((p) => p.id == 'team')) throw const CoreException(409, 'pack "team" is already added');
    _packs.add(_team);
    templatesForPacks.insert(
      0,
      const TemplateInfo(
        id: 'team/risk-review',
        name: 'Release risk review',
        workType: WorkTypeId.analyze,
        source: 'pack',
        description: 'The review every release gets.',
        prompts: ['review'],
        pack: 'team',
        mark: 'required',
      ),
    );
    demoMcpServers.add(
      const McpServer(
        name: 'team/github',
        command: 'npx',
        args: ['-y', '@modelcontextprotocol/server-github'],
        pack: 'team',
        trusted: false,
        mark: 'required',
      ),
    );
    return _team;
  }

  Future<List<PackInfo>> refreshPacks() async {
    packRefreshes++;
    return listPacks();
  }

  Future<void> removePack(String id) async {
    if (!_packs.any((p) => p.id == id)) throw CoreException(404, 'pack "$id" not found');
    _packs.removeWhere((p) => p.id == id);
    templatesForPacks.removeWhere((t) => t.pack == id);
    demoMcpServers.removeWhere((s) => s.pack == id);
  }

  Future<void> trustMcpServer(String name, {required bool trust}) async {
    final i = demoMcpServers.indexWhere((s) => s.name == name && s.pack.isNotEmpty);
    if (i < 0) throw CoreException(400, '"$name" is not an MCP server from a pack');
    final s = demoMcpServers[i];
    demoMcpServers[i] = McpServer(
      name: s.name,
      command: s.command,
      args: s.args,
      url: s.url,
      pack: s.pack,
      trusted: trust,
      mark: s.mark,
    );
  }
}
