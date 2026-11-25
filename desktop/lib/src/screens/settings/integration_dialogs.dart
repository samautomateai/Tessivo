import 'package:material_ui/material_ui.dart';

import '../../core/models.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/sizes.dart';

/// The fields each kind asks for: (key, label, hint).
const _fields = <String, List<(String, String, String)>>{
  'github': [
    ('project', 'Repository (owner/repo)', 'acme/shop'),
    ('base_url', 'API URL (GitHub Enterprise only)', 'https://github.acme.com/api/v3'),
  ],
  'gitlab': [('base_url', 'GitLab URL', 'https://gitlab.com'), ('project', 'Project (group/name or ID)', 'team/app')],
  'jira': [
    ('base_url', 'Jira URL', 'https://acme.atlassian.net'),
    ('project', 'Project key', 'OPS'),
    ('user', 'Account e-mail (Jira Cloud; empty for a personal access token)', ''),
    ('issue_type', 'Issue type (default Task)', 'Task'),
  ],
  'confluence': [
    ('base_url', 'Confluence URL', 'https://acme.atlassian.net/wiki'),
    ('space', 'Space key', 'ENG'),
    ('parent', 'Parent page ID (optional)', ''),
    ('user', 'Account e-mail (Confluence Cloud; empty for a personal access token)', ''),
  ],
  'notion': [('parent', 'Parent page ID', '0123456789abcdef0123456789abcdef')],
  'folder': [('path', 'Folder (absolute path)', '')],
};

/// Asks for a new integration: its kind, name and the kind's fields. The
/// token is asked for after it is added, and goes to the OS keychain.
class AddIntegrationDialog extends StatefulWidget {
  const AddIntegrationDialog({super.key});

  @override
  State<AddIntegrationDialog> createState() => _AddIntegrationDialogState();
}

class _AddIntegrationDialogState extends State<AddIntegrationDialog> {
  String _kind = 'github';
  final _name = TextEditingController();
  final Map<String, TextEditingController> _values = {};

  TextEditingController _field(String key) => _values[key] ??= TextEditingController();

  @override
  void dispose() {
    _name.dispose();
    for (final c in _values.values) {
      c.dispose();
    }
    super.dispose();
  }

  IntegrationInfo _result() {
    final name = _name.text.trim();
    final shown = {for (final f in _fields[_kind]!) f.$1};
    String only(String k) => shown.contains(k) ? (_values[k]?.text.trim() ?? '') : '';
    return IntegrationInfo(
      name: name,
      kind: _kind,
      baseUrl: only('base_url'),
      project: only('project'),
      user: only('user'),
      issueType: only('issue_type'),
      space: only('space'),
      parent: only('parent'),
      path: only('path'),
      tokenRef: _kind == 'folder' ? '' : 'TESSIVO_${name.toUpperCase().replaceAll('-', '_')}_TOKEN',
    );
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add an integration'),
    content: SizedBox(
      width: 520,
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TvDropdown<String>(
              key: const Key('integration-kind'),
              label: 'Kind',
              items: const [
                ('github', 'GitHub (issues)'),
                ('gitlab', 'GitLab (issues)'),
                ('jira', 'Jira (issues)'),
                ('confluence', 'Confluence (pages)'),
                ('notion', 'Notion (pages)'),
                ('folder', 'Folder (Markdown files)'),
              ],
              value: _kind,
              onChanged: (v) => setState(() => _kind = v),
            ),
            const SizedBox(height: TvSpace.s12),
            TvTextField(
              label: 'Name',
              fieldKey: const Key('integration-name'),
              controller: _name,
              hint: 'team-tracker',
              helper: 'Lower-case letters, digits, - or _.',
            ),
            for (final (key, label, hint) in _fields[_kind]!) ...[
              const SizedBox(height: TvSpace.s12),
              TvTextField(
                label: label,
                fieldKey: Key('integration-$key'),
                controller: _field(key),
                hint: hint.isEmpty ? null : hint,
              ),
            ],
          ],
        ),
      ),
    ),
    actions: [
      TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
      TvButton(
        'Add',
        key: const Key('integration-save'),
        kind: TvButtonKind.primary,
        onPressed: () => Navigator.of(context).pop(_result()),
      ),
    ],
  );
}

/// Asks for an integration's token; it goes to the OS keychain and is never shown again.
class TokenDialog extends StatefulWidget {
  const TokenDialog({super.key, required this.name});

  final String name;

  @override
  State<TokenDialog> createState() => _TokenDialogState();
}

class _TokenDialogState extends State<TokenDialog> {
  final _value = TextEditingController();

  @override
  void dispose() {
    _value.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('Token for ${widget.name}'),
    content: SizedBox(
      width: 460,
      child: TvTextField(
        label: 'Token',
        fieldKey: const Key('integration-token-value'),
        controller: _value,
        obscure: true,
        helper: 'Saved to your OS keychain by the core. It is never shown again.',
      ),
    ),
    actions: [
      TvButton('Later', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
      TvButton(
        'Save',
        key: const Key('integration-token-save'),
        kind: TvButtonKind.primary,
        onPressed: () => Navigator.of(context).pop(_value.text.trim()),
      ),
    ],
  );
}
