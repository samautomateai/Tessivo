part of 'app_state.dart';

/// Agent tool calls waiting for an answer, across notebooks.
extension ApprovalActions on AppState {
  /// Reloads pending approvals; errors keep the last list.
  Future<void> refreshApprovals() async {
    try {
      approvals = await core.pendingApprovals();
    } on Object {
      return;
    }
    _changed();
  }

  /// Answers an approval. [remember] is `run` or `notebook`.
  Future<void> answer(PendingApproval a, {required bool allow, String? remember}) =>
      _act(() => core.answerApproval(a.runId, a.approvalId, allow: allow, remember: remember), a);

  /// Resumes an interrupted run in the core.
  Future<void> resumeRun(PendingApproval a) => _act(() => core.resumeRun(a.runId), a);

  /// Cancels a paused or interrupted run.
  Future<void> cancelRun(PendingApproval a) => _act(() => core.cancelRun(a.runId), a);

  /// Runs an approval action; a refusal (expired, already answered, in use)
  /// becomes a notice, and the list is reloaded either way.
  Future<void> _act(Future<void> Function() action, PendingApproval a) async {
    try {
      await action();
      paused = [
        for (final p in paused)
          if (p.runId != a.runId || p.approvalId != a.approvalId) p,
      ];
    } on CoreException catch (e) {
      notices.add('Could not update ${a.tool.isEmpty ? 'the run' : a.tool}: ${e.message}');
    }
    await refreshApprovals();
    _changed();
  }
}
