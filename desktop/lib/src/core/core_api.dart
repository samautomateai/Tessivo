import 'dart:typed_data';

import 'chat_models.dart';
import 'mcp_models.dart';
import 'model_check.dart';
import 'models.dart';
import 'pack_models.dart';
import 'prompt_library.dart';
import 'research_models.dart';
import 'thread_models.dart';
import 'usage.dart';

/// The desktop app's view of the Go core. `HttpCoreClient` talks to the local
/// API (docs/api.md). Tests use a fake (test/support/fake_core).
abstract interface class CoreClient {
  Future<List<WorkType>> listWorkTypes();
  Future<List<Notebook>> listNotebooks();
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType});

  /// Renames, archives, restores, tags, pins or sets the watch mode (off, notify, run); null fields stay.
  Future<Notebook> updateNotebook(
    String notebookId, {
    String? name,
    bool? archived,
    List<String>? tags,
    bool? pinned,
    String? watch,
    WorkTypeId? workType,
  });

  /// Deletes a notebook and everything in it (409 while a run is active or paused).
  Future<void> deleteNotebook(String notebookId);
  Future<Notebook> setContextEnabled({required String notebookId, required String contextId, required bool enabled});

  /// Runs the notebook; [promptName] runs a saved prompt with its own settings.
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
    String? promptName,
    String? model,
    String? chain,
  });

  /// The run behind an output: its prompt, variables and model.
  Future<RunInfo> outputRun(String notebookId, String outputId);

  /// An output compared with the version it revised (or [against]).
  Future<OutputDiffInfo> diffOutput(String notebookId, String outputId, {String? against});
  Future<List<SavedPrompt>> listPrompts(String notebookId);
  Future<SavedPrompt> savePrompt(String notebookId, SavedPrompt prompt);
  Future<void> deletePrompt(String notebookId, String name);

  /// A saved prompt's versions, the current one first (PR-4).
  Future<List<PromptVersion>> promptVersions(String notebookId, String name);

  /// Saves an earlier version as the prompt's new current version.
  Future<SavedPrompt> restorePromptVersion(String notebookId, String name, int version);
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType, String? notebookId});
  Future<Notebook> getNotebook(String notebookId);
  Future<Notebook> addContext(String notebookId, String path);

  /// Adds a web page, fetched on every run.
  Future<Notebook> addUrlContext(String notebookId, String url, {String label});

  /// Saves pasted text as a source.
  Future<Notebook> addTextContext(String notebookId, String text, {String label});

  /// Adds a repository's changes since [base], read with git on every run.
  Future<Notebook> addGitDiffContext(String notebookId, String repo, String base, {String label});
  Future<Notebook> setMemory(String notebookId, List<String> memory);
  Future<OutputDetail> getOutput(String notebookId, String outputId);

  /// An output as a file: format md or docx.
  Future<Uint8List> exportOutput(String notebookId, String outputId, String format);

  /// A zip for bug reports (no keys, notebook names or content).
  Future<Uint8List> diagnostics();

  /// Notebooks, outputs, prompts, research notebooks and notes matching every word.
  Future<List<SearchHit>> search(String query);
  Future<SettingsInfo> getSettings();

  /// The starter prompts, for one work type (analyze, summarize, …) or all.
  Future<List<LibraryPrompt>> promptLibrary({String? workType});

  /// Today's token use and the limits.
  Future<UsageInfo> usage();

  /// Sends one tiny request to every model behind an alias or provider/model.
  Future<List<ModelCheck>> testModel(String model);

  /// Saves providers, aliases, defaults and limits (a SettingsDraft body).
  Future<SettingsInfo> updateSettings(Map<String, Object?> body);

  /// Whether an API key is available; its value never leaves the core.
  Future<bool> secretSet(String ref);
  Future<void> setSecret(String ref, String value);
  Future<void> deleteSecret(String ref);
  Future<Map<String, List<String>>> listModels({String? provider});
  Future<List<ScheduleInfo>> listSchedules(String notebookId);
  Future<ScheduleInfo> addSchedule(String notebookId, {required String cron, String prompt, String missed});
  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled});
  Future<void> deleteSchedule(String notebookId, String scheduleId);

  Future<List<TemplateInfo>> listTemplates({WorkTypeId? workType});
  Future<Notebook> createFromTemplate(String templateId, {String? name});
  Future<TemplateInfo> saveAsTemplate(String notebookId, {required String name, String description});
  Future<void> deleteTemplate(String templateId);

  Future<List<ChatSummary>> listChats({String query, bool archived});
  Future<ChatSession> createChat({String title, String model, String thinkingEffort});
  Future<ChatSession> getChat(String chatId);
  Future<ChatSession> updateChat(
    String chatId, {
    String? title,
    bool? pinned,
    bool? archived,
    String? model,
    String? thinkingEffort,
    List<String>? mcpServers,
  });

  /// Answers a tool call a chat reply is waiting on ([ChatApprovalNeeded]).
  Future<void> answerChatTool(String chatId, String callId, {required bool allow, bool remember});
  Future<void> deleteChat(String chatId);

  /// Streams a reply. Cancelling the subscription stops it; the core keeps the partial reply.
  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments});
  Future<List<String>> chatMemory();
  Future<List<String>> setChatMemory(List<String> memory);
  Future<RepeatablePlan> planRepeatable(String chatId);
  Future<Notebook> makeRepeatable(String chatId, {String? name, WorkTypeId? workType, String? templateId});
  Future<ChatOutput> saveReply(String chatId, String messageId, WorkTypeId workType);

  /// Live notifications (scheduled runs). Never completes while the core runs.
  Stream<CoreEvent> events();

  /// Agent tool calls waiting for an answer, across all notebooks.
  Future<List<PendingApproval>> pendingApprovals();

  /// Answers an approval; the core then resumes the run in the background.
  /// [remember] is `run` or `notebook` to allow later calls of the same tool.
  Future<void> answerApproval(String runId, String approvalId, {required bool allow, String? remember});

  /// Stops an agent run (running, paused or interrupted).
  Future<void> cancelRun(String runId);

  /// Resumes an interrupted run (or a paused one whose approvals are answered).
  Future<void> resumeRun(String runId);

  // Output threads (docs/api.md, "Output threads").
  Future<ChatSession> getThread(String notebookId, String outputId);
  Future<ChatSession> setThreadSettings(String notebookId, String outputId, {String? model, String? thinkingEffort});
  Stream<ThreadEvent> sendThread(String notebookId, String outputId, String text, {List<ChatAttachment> attachments});
  Future<List<Suggestion>> listSuggestions(String notebookId, String outputId);
  Future<List<Suggestion>> generateSuggestions(String notebookId, String outputId);
  Future<({Suggestion suggestion, String? promptName})> applySuggestion(
    String notebookId,
    String outputId,
    String suggestionId,
  );
  Future<Suggestion> dismissSuggestion(String notebookId, String outputId, String suggestionId);
  Future<OutputSummary> newVersion(String notebookId, String outputId, {String? fromMessageId});

  /// Saves [data], the output's structured result as edited, as a new version (no model call).
  Future<OutputSummary> editOutput(String notebookId, String outputId, Map<String, Object?> data);

  /// The tracker-ready issues an Analyze output's findings make (WT-5).
  Future<List<IssueInfo>> outputIssues(String notebookId, String outputId);

  /// Files findings ([findings]: indexes; empty: all) as issues; sent ones are skipped unless [again].
  Future<PushResult> pushIssues(
    String notebookId,
    String outputId,
    String integration, {
    List<int> findings = const [],
    bool again = false,
  });

  /// Publishes the output to a folder, Confluence or Notion.
  Future<Delivery> publishOutput(String notebookId, String outputId, String integration);

  /// What was sent from an output, oldest first.
  Future<List<Delivery>> deliveries(String notebookId, String outputId);

  Future<List<IntegrationInfo>> listIntegrations();
  Future<IntegrationInfo> addIntegration(IntegrationInfo integration);
  Future<void> removeIntegration(String name);

  /// Checks an integration's connection and token; throws with the reason.
  Future<void> testIntegration(String name);

  /// Settings profiles (SET-10).
  Future<ProfilesInfo> profiles();

  /// Saves the current providers, aliases, defaults and limits as profile [name] and makes it active.
  Future<ProfilesInfo> saveProfile(String name);

  /// Switches to profile [name]; the active one keeps the current settings first.
  Future<ProfilesInfo> useProfile(String name);
  Future<ProfilesInfo> deleteProfile(String name);

  /// The work type the notebook's sources point to (WT-7), while it has no outputs.
  Future<WorkTypeSuggestion> workTypeSuggestion(String notebookId);

  // MCP servers (docs/api.md, "MCP servers").
  Future<List<McpServer>> listMcpServers();
  Future<McpServer> addMcpServer(McpServer server);
  Future<void> removeMcpServer(String name);
  Future<McpTestResult> testMcpServer(String name);
  Future<Notebook> setNotebookMcp(String notebookId, List<String> servers);

  /// Replaces the notebook's run_command allow-list (agent mode).
  Future<Notebook> setNotebookCommands(String notebookId, List<CommandRule> commands);
  Future<Notebook> addMcpContext(String notebookId, String server, String uri);

  // Archives (NB-4): the core builds and reads them; the app streams files.
  Future<void> exportNotebook(String notebookId, String toPath, {bool outputs = false});
  Future<Notebook> importNotebook(String fromPath);
  Future<Notebook> relinkContext(String notebookId, String contextId, String path);
  Future<void> exportResearch(String id, String toPath);
  Future<ResearchNotebook> importResearch(String fromPath);

  // Team packs (docs/api.md, "Packs"): templates, prompts and MCP servers a team shares.
  Future<List<PackInfo>> listPacks();
  Future<PackInfo> addPack(String source, {String branch = ''});
  Future<List<PackInfo>> refreshPacks();
  Future<void> removePack(String id);

  /// Trusts or stops trusting a pack MCP server (`<pack>/<server>`).
  Future<void> trustMcpServer(String name, {required bool trust});

  // Research notebooks (docs/api.md, "Research notebooks").
  Future<List<ResearchNotebook>> listResearch();
  Future<ResearchNotebook> createResearch(String name, {String description});
  Future<ResearchNotebook> getResearch(String id);
  Future<void> deleteResearch(String id);
  Future<ResearchNotebook> addResearchSource(String id, {String? path, String? server, String? uri});
  Future<ResearchNotebook> setSourceLevel(String id, String sourceId, String level);
  Future<ResearchNotebook> removeResearchSource(String id, String sourceId);
  Future<void> refreshSummary(String id, String sourceId);
  Future<ResearchSourceText> researchSource(String id, String sourceId);
  Future<ContextTotals> researchContext(String id);
  Future<List<ChatSummary>> listResearchChats(String id);
  Future<ChatSession> createResearchChat(String id, {String title});
  Future<ResearchChat> getResearchChat(String id, String chatId);
  Stream<ResearchEvent> sendResearch(String id, String chatId, String text);
  Future<List<ResearchNote>> researchNotes(String id);
  Future<ResearchNote> saveResearchNote(String id, ResearchNote note);
  Future<void> deleteResearchNote(String id, String noteId);
  Future<ResearchNote> noteFromMessage(String id, String chatId, String messageId);
}
