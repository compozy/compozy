# Loop Effect Daemon Policy Implementation Plan

> For agentic workers: execute this plan with test-first changes, canonical suites, independent review, and evidence-backed verification.

**Goal:** Deliver permitted Loop terminal tool effects under a daemon-owned, same-workspace scope without resolving the synthetic `loop-effect` audit label as an authored agent.

**Architecture:** Preserve the trusted actor kind through native tool policy resolution, bypass only authored-agent policy for daemon actors, and teach the native workspace binder to authorize daemon scopes against their exact workspace identity. Keep the registry, tool policy evaluator, workspace policy, and relay delivery path otherwise unchanged.

**Tech stack:** Go 1.26, Compozy daemon native tool registry, Loop effect outbox/relay, `workspaceaccess` policy, canonical Go unit/integration suites, Mage/Make verification.

**Design:** `docs/superpowers/specs/2026-08-14-loop-effect-daemon-policy-design.md`

**Issue:** [#403](https://github.com/compozy/compozy/issues/403)

## Global constraints

- Follow repository error wrapping, naming, and test conventions.
- Extend existing canonical suites; do not add a duplicate standalone suite.
- Preserve the synthetic `loop-effect` label for attribution, but never materialize it as a public agent.
- Preserve global/workspace policy, source trust, permission ceilings, approval behavior, correlation IDs, and cross-workspace denial.
- Do not change public ToolIDs, schemas, descriptors, config, extension formats, or generated contracts.
- Record RED evidence before modifying production code.
- Run `make gate-full` exactly once, only after all implementation and QA writes are complete.

## Task 1: Prove both authorization barriers

**Files:**

- Modify: `internal/daemon/native_tools_test.go`
- Modify: `internal/daemon/loop_effect_relay_test.go`

**Invariant:** A trusted daemon-owned tool call uses workspace policy without an authored agent or caller session, and cannot escape its recorded workspace.

**Owning layer:** daemon service integration.

**Existing suites:** `TestDaemonNativeRuntimePolicyResolver`, `TestDaemonNativeTools`, and `TestLoopEffectRelayShouldDrainCommittedEntriesInIsolation`.

1. Add a policy resolver case with `ActorKind: daemon`, a workspace ID, and `AgentName: loop-effect`. Assert that the resolver does not call the authored agent resolver and still returns the configured workspace/global policy.
2. Add the negative companion: the same unknown name in a non-daemon agent/session scope must retain `workspace.ErrAgentNotAvailable`.
3. Extend the native workspace input binding subtest with a daemon same-workspace call that succeeds without a session.
4. Add negative companions for daemon foreign-workspace and missing-workspace scopes.
5. Add a relay integration case that uses a real native runtime registry, real policy resolver, and real workspace input binder to deliver `compozy__session_prompt` to a target session in the same workspace. Assert one prompt, one successful acknowledgement, the daemon actor kind, and stable delivery correlation.
6. Run the focused suite and capture the exact pre-fix failures:

```bash
mise exec -- go test ./internal/daemon -run 'Test(DaemonNativeRuntimePolicyResolver|DaemonNativeTools|LoopEffectRelay)' -count=1
```

Expected: the new cases fail with the authored-agent lookup and session-ID workspace authorization errors.

## Task 2: Implement the actor-aware root fix

**Files:**

- Modify: `internal/daemon/tool_policy_resolver.go`
- Modify: `internal/daemon/native_workspace_input_authorizer.go`
- Test: `internal/daemon/native_tools_test.go`
- Test: `internal/daemon/loop_effect_relay_test.go`

1. Preserve trimmed `ActorKind` in `normalizeToolPolicyScope`.
2. Skip authored-agent policy resolution only when `ActorKind` is `workspaceaccess.ActorDaemon`; continue applying every other policy input.
3. In `nativeWorkspaceAccessActor`, construct an `ActorDaemon` reference from a daemon scope with a non-empty workspace ID before the session path.
4. Reject daemon scopes with no workspace ID. Do not synthesize a session or add an approval escape hatch.
5. Run the focused tests until green:

```bash
mise exec -- go test ./internal/daemon -run 'Test(DaemonNativeRuntimePolicyResolver|DaemonNativeTools|LoopEffectRelay)' -count=1
mise exec -- go test -race ./internal/daemon -run 'Test(DaemonNativeRuntimePolicyResolver|DaemonNativeTools|LoopEffectRelay)' -count=1
```

## Task 3: Prove real Loop delivery and update QA ownership

**Files:**

- Modify: `internal/daemon/loop_node_lifecycle_e2e_integration_test.go`
- Modify: `docs/qa/scenarios/LP-terminal-outcome-notification.md`

**Invariant:** A terminal outcome committed by a real Loop run executes its declared permitted native tool effect and records the successful effect result once.

**Owning layer:** existing daemon Loop integration and LP QA scenario.

1. Extend the existing lifecycle E2E suite with a minimal terminal tool effect through real daemon wiring. Use a read-only native tool and assert the durable effect result/outbox state rather than a fake return.
2. Add a negative assertion that no authored/public `loop-effect` agent is required or exposed.
3. Update `LP-terminal-outcome-notification` with issue #403, fix status, and the intended evidence path. Do not create a new scenario.
4. Run the focused integration case:

```bash
mise exec -- go test -tags=integration ./internal/daemon -run 'TestLoopNodeLifecycle' -count=1 -v
```

5. Run daemon convention checks and the complete race suite:

```bash
mise exec -- go test -race ./internal/daemon -count=1
```

## Task 4: Build, isolated QA, and ship

1. Run `make build` and record version, commit, and checksum.
2. Create a fresh isolated lab with `eng-qa-bootstrap`; use isolated HOME, HTTP port, and UDS.
3. Reproduce the generic Loop from issue #403 with a terminal native tool effect. Confirm:
   - the run reaches its terminal state;
   - the effect outbox result is successful;
   - the target native effect occurs exactly once in the observed delivery;
   - no `agent not available` or session-ID authorization error occurs;
   - a foreign-workspace target remains denied;
   - no public `loop-effect` agent appears.
4. Update the existing LP scenario/report with structured CLI/HTTP/UDS evidence and complete mandatory teardown with `"clean": true`.
5. Run scoped format, lint, convention, and diff checks. Run `deslop`, review the complete diff, and resolve every real finding.
6. After all writes are complete, run exactly once:

```bash
mise exec -- make gate-full
make gate-status
```

7. Create conventional local commits, rebase on the latest `upstream/main` if it advanced, push the feature branch, and open a PR titled:

```text
fix: authorize daemon-owned loop tool effects
```

The PR must use the repository template, include `Fixes #403`, list exact verification evidence, include the Compozy Impact Audit, and disclose AI assistance plus independent verification.
