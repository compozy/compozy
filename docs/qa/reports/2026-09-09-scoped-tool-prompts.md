# Scoped startup tool guidance

Issue: [#563](https://github.com/compozy/compozy/issues/563). Scenario: [ET-049](../scenarios/ET-049.md).

## Verified behavior

The composed startup prompt includes the bundled `compozy` router once for tools-enabled
interactive sessions, system task workers, dream curators, coordinators, and ordinary spawned
workers. It does not inline either tool reference manual. The native skill-view suite verifies
that both complete manuals remain readable on demand through the existing registry, including
workspace-scoped artifact continuations when the configured result budget offloads the envelope.
Structured resource content remains complete; display text preserves intentional secret redaction.

Memory extraction is tested through its actual `spawnExtractorSession` caller, using a custom
agent and the real session manager. Checkpoint summarization is tested through `Summarize`, also
with a custom agent. Both omit the router because their durable role identifies a routine that
only transforms supplied input. A stopped and resumed extractor preserves that selection.
Automatic title generation has the same supplied-input-only contract and is covered in the same
assembly matrix using its existing `SpawnRoleAutoTitle`. Unknown roles and older unmarked
checkpoint sessions conservatively retain the router.

## Reproduction and measurement

Before the fix, the existing composed-assembler suite failed the router contract: a minimal
startup was 53,850 bytes, with 53,836 bytes of bundled tool reference source. The test output
reported the missing router before production code changed.

The integration pass measured complete prompts at the session driver's startup boundary:

| Session | Assembled bytes | Tool router |
| --- | ---: | --- |
| Interactive | 13,987 | once |
| System worker | 13,991 | once |
| Dream curator | 13,989 | once |
| Spawned worker | 13,993 | once |
| Memory extractor via Spawn | 3,600 | omitted |
| Resumed extractor | 3,409 | omitted |
| Checkpoint summary | 3,402 | omitted |

These are fixture measurements, not workload token or billing estimates. The recorded router
body was 10,648 bytes; formatting and editorial changes can alter byte counts without altering
selection. The follow-up composed-assembler pass measured an 11,585-byte router and 11,599-byte
minimal capable startup after the continuation guidance was added; the input-only cases contained
only the 12-byte base fixture. The table does not imply that every session in one class needs no tools. Coordinator
selection, disabled tools, role normalization, and unknown roles are covered by the owning
composed-assembler matrix.

## Verification

- `CGO_ENABLED=1 go test -race ./internal/daemon -run 'TestComposedAssembler|TestHarnessContextResolver|TestDaemonCheckpointSummarizer|TestForkedMemoryExtractor' -count=1` passed.
- `CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestHarnessContextIntegration' -count=1 -v` passed all four scenarios, including real SQLite creation and resume. The first pass exposed an old task fixture missing its required profile identity and an invalid function-value `reflect.DeepEqual`; the fixture now supplies the profile and resume compares filter behavior as well as all other policy fields.
- `TestDaemonE2EAutomationPromptTriggerCreatesCompletedSystemSession` passed with the real daemon and ACP mock driver in subprocesses (65.48 seconds for the scenario), using the shared verification lock.
- The capped follow-up checks passed for boot flag combinations, the complete role matrix
  (including auto-title), and native full-resource recovery. The latter passed in 40.600 seconds
  with `go test -race -p=1 -parallel=4 ./internal/daemon -run
  '^TestDaemonNativeTools/Should_dispatch_skill_catalog_tools_through_the_real_skill_registry$'
  -count=1 -v`; both direct and retained-envelope paths returned all 19,804 and 34,032 structured
  manual bytes. The display path retained the two redaction records for the tools-and-skills manual.
- Native resource coverage extends the existing `TestDaemonNativeTools/Should_dispatch_skill_catalog_tools_through_the_real_skill_registry` case to both manuals, preserving full structured-content checks and intentional display redaction. A 32 KiB fixture
  budget exercises the existing artifact offload and native paged-read continuation without
  raising any production limit.
- Integration homes are allocated by `integrationHomePaths` under `t.TempDir`; the suite isolates `HOME`, `COMPOZY_HOME`, and sockets and joins its owned runtime during cleanup. No operator provider credentials are used. The targeted teardown reported `TEARDOWN_ALL_CLEAN=true`; its captured evidence is retained locally at `.cache/issue-563/teardown.json`.

The integration suite uses the existing driver fixture with real session storage. It does not
measure real-model compliance with reference reading or provider billing. The PR records the
additional subprocess runtime check, pinned local gate, current-head CI, and review dispositions.

## Compatibility and scope

No schema, config key, tool ID, descriptor, permission rule, hosted MCP behavior, or workspace
boundary changes. The existing lineage `spawn_role` field carries the checkpoint-summary role;
older data remains readable without migration. Startup and turn resolution receive the same
role. This is prompt selection, not an authorization boundary. Memory/dreaming defaults and the
extractor parser/lifecycle remain owned by #561 and are unchanged here. Memory-dependent harness
fixtures explicitly opt in.
