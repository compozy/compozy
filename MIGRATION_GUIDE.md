# Migrate from Compozy v0.2.15 to CompozyOS v0.3

CompozyOS v0.3 is a hard cut, not an in-place compatibility release. It keeps the `compozy` binary
and the `.compozy/tasks/` task format, but replaces the workflow runner with a daemon-owned agent
operating system. Runtime databases, command names, extension contracts, and review behavior do not
carry forward automatically.

This guide uses v0.2.15 (`8f8908afd70c731b815e20282bacad05aa026827`) as the legacy baseline.
Back up your global and workspace `.compozy/` directories before changing installations.

<a id="command-map"></a>

## 1. Command map

Install the v0.3 beta through one of the beta paths. Replace `beta.N` with the published prerelease
number when using the versioned Go command. Homebrew still serves v0.2 during the beta window and is
therefore not an upgrade path.

```bash
npm install -g @compozy/cli@beta

curl -fsSL https://compozy.com/install.sh | \
  sh -s -- --version v0.3.0-beta.N

go install github.com/compozy/compozy@v0.3.0-beta.N
```

The primary delivery mappings are:

| v0.2.15                                                                             | v0.3 beta                                                                                           | Notes                                                                                                                                    |
| ----------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `compozy tasks run demo`                                                            | `compozy loop run --workspace <ref> --name software-delivery --input slug=demo`                     | Uses the bundled delivery Loop and the existing `.compozy/tasks/demo/` tree.                                                             |
| `--task-runtime type=frontend,ide=codex,model=gpt-5.5-codex,reasoning-effort=xhigh` | `--runtime type=frontend:codex/gpt-5.5-codex@xhigh`                                                 | `--runtime` is repeatable. It also accepts `worker=`, `judge=`, `id=`, and `complexity=` selectors.                                      |
| `--dry-run`                                                                         | `--dry-run`                                                                                         | The v0.3 plan reports effective input values and origins and creates no run.                                                             |
| `--auto-commit`                                                                     | `--input auto_commit=true`                                                                          | Supported by both bundled dev-cycle Loops. Neither Loop pushes.                                                                          |
| `compozy reviews fix <task>`                                                        | `compozy loop run --workspace <ref> --name review-and-fix --input task_name=<task>`                 | Semantic change: a Compozy reviewer agent authors a new round before fixers run. Existing provider-fetched rounds are not imported.      |
| `compozy reviews watch <task> --pr <N>`                                             | Same `review-and-fix` command above                                                                 | Semantic change: there is no PR polling, CodeRabbit fetch, thread resolution, or push tail. `--pr` and `--provider` have no replacement. |
| `compozy exec "prompt"`                                                             | `compozy session new --workspace <ref> --agent <name>`, then `compozy session prompt <id> "prompt"` | v0.3 makes the durable session explicit. Use `session events`, `status`, `wait`, `stop`, and `resume` for lifecycle control.             |
| `compozy agents list` / `agents inspect <name>`                                     | `compozy agent list` / `compozy agent info <name>`                                                  | v0.3 agent definitions are daemon-managed resources, not setup assets.                                                                   |
| `compozy mcp-serve`                                                                 | `compozy mcp serve --workspace <ref>`                                                               | Serves the workspace-bound Compozy Host API.                                                                                             |
| `compozy workspaces list/show/register/unregister`                                  | `compozy workspace list/info/add/remove`                                                            | Use IDs or explicit roots returned by the daemon. The old `resolve` verb has no direct command replacement.                              |
| `compozy daemon start`                                                              | `compozy install`, then `compozy status`                                                            | `install` bootstraps the supervised runtime. Use `daemon relaunch` for an installed daemon.                                              |
| `compozy daemon status/stop`                                                        | `compozy status` / `compozy daemon stop`                                                            | Structured output is available with `-o json`.                                                                                           |
| `compozy upgrade`                                                                   | `compozy update`                                                                                    | A beta build tracks newer v0.3 beta releases, never the v0.2 stable line.                                                                |
| `compozy ext list/install/enable/disable/doctor`                                    | `compozy extension list/install/enable/disable/status` plus `compozy doctor`                        | Extension manifests and SDK contracts are API-incompatible; reinstall or port extensions instead of reusing old directories.             |
| `compozy runs watch <id>`                                                           | `compozy loop status --workspace <ref> --run-id <id>` or the printed `/loop-runs/<id>` URL          | Legacy run IDs do not exist in the new database. For sessions use `session status/events/wait`.                                          |
| `compozy runs attach <id>`                                                          | No TUI replacement                                                                                  | Use structured status/events and the web run page.                                                                                       |
| `compozy runs purge`                                                                | No bulk compatibility verb                                                                          | Start with clean v0.3 runtime state. Remove individual session history with `session remove` when appropriate.                           |
| `compozy sync`                                                                      | No replacement required                                                                             | Loop task import and daemon resource registration happen through their owning runtime surfaces.                                          |
| `compozy archive`                                                                   | No replacement                                                                                      | Move completed task folders manually only after preserving the evidence your repository requires.                                        |
| `compozy tasks validate`                                                            | Removed; no `loop validate` replacement                                                             | `loop validate` validates a Loop definition, not legacy task files. Do not substitute it in scripts or bundled process guidance.         |
| `compozy migrate`                                                                   | Run the v0.2 command before upgrading, only for XML-era task/review artifacts                       | This is the old XML-to-frontmatter converter. It is not `compozy migrate config`.                                                        |

The complete v0.2 flag-family disposition is:

| Legacy flag family                                                                                                                                                          | v0.3 disposition                                                                                                                                                      |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `tasks run`: positional slug, `--name`                                                                                                                                      | `--name software-delivery --input slug=<slug>`; the Loop name and task slug are separate values.                                                                      |
| `--ide`, `--model`, `--reasoning-effort`, repeatable `--task-runtime`                                                                                                       | Express worker/judge or task rules with repeatable `--runtime`. Unsupported legacy providers are not aliased.                                                         |
| `--format`, common structured output                                                                                                                                        | Use global `-o` with `human`, `json`, `jsonl`, or `toon`.                                                                                                             |
| `--dry-run`, `--auto-commit`                                                                                                                                                | Retained as `--dry-run` and `--input auto_commit=<bool>`.                                                                                                             |
| `--multiple`, `--parallel`, `--parallel-limit`                                                                                                                              | Deferred. Start separate Loop runs in explicitly isolated workspaces/worktrees.                                                                                       |
| `--parallel-tasks` and hidden conflict-resolver flags                                                                                                                       | Deferred. The bundled Loop intentionally executes its task fan-out with `max_parallel: 1`.                                                                            |
| `--include-completed`, `--recursive`, `--skip-validation`, `--force`                                                                                                        | Removed. The bundled importer owns task selection; there is no bypass flag. Fork and publish a custom Loop when a different contract is required.                     |
| `--attach`, `--ui`, `--stream`, `--detach`                                                                                                                                  | TUI attach is removed. Use the final-line web URL, `loop status`, run events over HTTP/UDS/SSE, or structured output.                                                 |
| `--add-dir`, `--tail-lines`, `--access-mode`, `--timeout`, `--max-retries`, `--retry-backoff-multiplier`                                                                    | No direct invocation flags. Model execution policy in agent definitions, sandbox/config policy, or a custom Loop node's `timeout` and `retry`.                        |
| `--recovery`, `--no-recovery`, `--recovery-ide`, `--recovery-model`, `--recovery-reasoning`, `--recovery-max-attempts`                                                      | Removed. The legacy recovery agent has no v0.3 replacement. Use Loop retry/no-progress/terminal state and operator-driven reruns.                                     |
| `reviews fetch`: `--provider`, `--pr`, `--name`, `--round`                                                                                                                  | External fetch is removed. `task_name` is the only required review target; round allocation is automatic.                                                             |
| `reviews fix`: `--name`, `--round`, `--agent`, `--batch-size`, `--concurrent`, `--include-resolved`, common runtime/recovery/attach flags                                   | Use `task_name`, optional `fixer`, optional `auto_commit`, and `--runtime`; other flags have no direct replacement.                                                   |
| `reviews watch`: fetch flags plus `--auto-push`, `--push-remote`, `--push-branch`, `--poll-interval`, `--quiet-period`, `--max-rounds`, `--review-timeout`, `--until-clean` | Removed with provider watch semantics. Run `review-and-fix` once; push is operator or outer-Loop work.                                                                |
| `exec`: prompt/stdin, `--agent`, `--prompt-file`, `--format`, `--persist`, `--run-id`, `--tui`, `--extensions`, runtime/retry/stall flags                                   | Split across `session new`, `session prompt`, session lifecycle commands, agent/provider config, and extension policy. There is no flag-for-flag compatibility layer. |
| `setup`: `--agent`, `--skill`, `--global`, `--copy`, `--list`, `--yes`                                                                                                      | The external-CLI symlink/copy installer is removed. Use daemon-managed agent/skill surfaces and the bundled dev-cycle extension.                                      |
| `migrate`, `sync`, `archive`: `--root-dir`, `--name`, `--tasks-dir`, `--reviews-dir`, `--dry-run`, `--format`                                                               | Only the old XML converter remains relevant before upgrade. Sync/archive behavior has no v0.3 command equivalent.                                                     |

<a id="config-translation"></a>

## 2. Configuration translation contract

The live config migrator is deferred, so this is both the manual mapping and the future Task 14
contract. Apply global and workspace changes separately. v0.3 precedence is per-run value, then
workspace `[loops.inputs.<loop>]`, then global input default, then the Loop definition default.
Explicit `false`, `0`, and valid empty strings remain present values.

| v0.2.15 source                                                                                                                                                 | v0.3 target or disposition                                                                                                                                           |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `defaults.ide`                                                                                                                                                 | Expand, after provider mapping, to `loops.defaults.{delivery,watch}.runtime_defaults.{worker,judge}.provider`.                                                       |
| `defaults.model`                                                                                                                                               | Expand to `loops.defaults.{delivery,watch}.runtime_defaults.{worker,judge}.model`.                                                                                   |
| `defaults.reasoning_effort`                                                                                                                                    | Expand to `loops.defaults.{delivery,watch}.runtime_defaults.{worker,judge}.reasoning` for `low`, `medium`, `high`, `xhigh`, or `max`.                                |
| `defaults.timeout`                                                                                                                                             | `session.limits.timeout`.                                                                                                                                            |
| `defaults.auto_commit`                                                                                                                                         | `loops.inputs.software-delivery.auto_commit` and `loops.inputs.review-and-fix.auto_commit`.                                                                          |
| `defaults.by_complexity.<level>.<field>`                                                                                                                       | Ordered delivery `runtime_rules` with `match.complexity`; levels are `low`, `medium`, `high`, and `critical`, and fields are `ide`, `model`, and `reasoning_effort`. |
| `tasks.run.task_runtime_rules[]`                                                                                                                               | Ordered delivery `runtime_rules` with `match.type`. The persisted legacy schema supported type selectors only.                                                       |
| `defaults.{output_format,access_mode,tail_lines,add_dirs,max_retries,retry_backoff_multiplier}`                                                                | Dropped; no runtime-config destination.                                                                                                                              |
| `defaults.stall.{enabled,timeout,child_timeout,terminal_command_timeout,retries}`                                                                              | Dropped; use Loop retry/no-progress/terminal-state contracts instead.                                                                                                |
| `tasks.types`                                                                                                                                                  | Dropped.                                                                                                                                                             |
| `tasks.run.{include_completed,recursive,output_format,run_multiple_mode,run_multiple_parallel_limit,tui}`                                                      | Dropped; the bundled Loop owns task selection and has no TUI.                                                                                                        |
| `tasks.run.parallel.{enabled,max_concurrency}`                                                                                                                 | Dropped; dependency-wave execution is deferred.                                                                                                                      |
| `tasks.run.parallel.conflict_resolver.{enabled,ide,model,reasoning_effort,max_attempts,validation_command}`                                                    | Dropped; the worktree conflict resolver is deferred with parallel task execution.                                                                                    |
| `fix_reviews.{concurrent,batch_size,include_resolved,output_format,tui}`                                                                                       | Dropped; v0.3 review is agent-authored and the bundled Loop owns batching.                                                                                           |
| `fetch_reviews.{provider,nitpicks}`                                                                                                                            | Dropped; v0.3 fetches no external reviews.                                                                                                                           |
| `watch_reviews.{auto_push,push_remote,poll_interval,quiet_period,max_rounds,review_timeout,until_clean,push_branch}`                                           | Dropped; there is no provider watch, thread resolution, or push tail.                                                                                                |
| `exec.{ide,model,output_format,reasoning_effort,access_mode,timeout,tail_lines,add_dirs,auto_commit,max_retries,retry_backoff_multiplier,verbose,tui,persist}` | Dropped; create and configure explicit sessions instead.                                                                                                             |
| `exec.stall.{enabled,timeout,child_timeout,terminal_command_timeout,retries}`                                                                                  | Dropped.                                                                                                                                                             |
| `runs.{default_attach_mode,keep_terminal_days,keep_max,shutdown_drain_timeout}`                                                                                | Dropped; legacy history is not imported.                                                                                                                             |
| `recovery.{enabled,ide,model,reasoning_effort,max_attempts}`                                                                                                   | Dropped; the recovery agent is removed.                                                                                                                              |
| `sound.{enabled,on_completed,on_failed,on_parked}`                                                                                                             | Dropped; no v0.3 destination.                                                                                                                                        |

Provider mapping is exact: `codex → codex`, `claude → claude`, `cursor-agent → cursor`,
`opencode → opencode`, `pi → pi`, `gemini → gemini`, `copilot → copilot`, and `kiro → kiro`.
Drop `droid`, `devin`, and legacy-only reasoning `ultra` as
`unsupported_destination_value`; never silently alias or weaken them. The v0.2 schema had no
persisted `defaults.provider` or `defaults.pr`, so do not fabricate either.

You can manage an input default manually now:

```bash
compozy config set loops.inputs.software-delivery.auto_commit false \
  --scope workspace --workspace /absolute/project/root

compozy config get loops.inputs.software-delivery.auto_commit \
  --workspace /absolute/project/root

compozy config unset loops.inputs.software-delivery.auto_commit \
  --scope workspace --workspace /absolute/project/root
```

HTTP and UDS expose exact-scope inspect, replace, set, and delete operations under
`/api/workspaces/{workspace_id}/loops/{name}/input-defaults`. Configured values are checked against
the named Loop only during dry-run or submission, because Loops can enroll dynamically.

<a id="config-migrator"></a>

## 3. Config migrator status

`compozy migrate config` does **not** ship in this task or in the current v0.3 beta. Backup,
idempotent rewrite, legacy-state first-boot probing, and the executable translation report are
deferred to Task 14. Do not run that command, build automation around it, or assume the daemon will
read v0.2 config as a fallback.

Until Task 14 ships:

1. Back up `~/.compozy/` and each `<workspace>/.compozy/` directory.
2. Translate the config manually with the table above into a fresh v0.3 config.
3. Move legacy-owned `agents/` and `extensions/` directories out of the colliding `.compozy/`
   namespace. Do not let v0.3 enroll their old formats.
4. Start v0.3 with clean runtime state; do not copy `global.db`, daemon sockets, locks, logs, run
   directories, or other v0.2 state into the new runtime.
5. Keep the backup until the beta journey and required task evidence have been verified.

The old v0.2 `compozy migrate` command is unrelated. It converts XML-tagged task and review
artifacts into frontmatter. If your task tree still contains `<task_context>` or `<review_context>`,
run the v0.2.15 converter **before** replacing the CLI. The future `migrate config` command is not a
replacement for that artifact conversion.

<a id="skills"></a>

## 4. Where the skills went

The v0.3 `dev-cycle` extension bundles exactly nine retained process skills:

- `cy-create-prd`
- `cy-create-techspec`
- `cy-create-tasks`
- `cy-execute-task`
- `cy-workflow-memory`
- `cy-review-round`
- `cy-fix-reviews`
- `cy-final-verify`
- `git-rebase`

They are versioned resources of the extension and are exposed through the runtime's skill/resource
surfaces. The legacy `compozy` skill is retired; the new official `skills/compozy/` guide is the
runtime and contributor manual, not one of the nine dev-cycle process skills.

The v0.2 `compozy setup` helper copied or symlinked public skills and reusable `AGENT.md` files into
external agent-CLI homes. v0.3 has no equivalent external-CLI installer. Use `compozy skill` for
daemon-managed local or marketplace skills, `compozy agent` for runtime agent definitions, and
`compozy extension` for versioned extension resources. Copying the old installed files forward does
not make them compatible.

Process guidance must not call `tasks validate`: that command is removed and `loop validate` checks
Loop definitions, not task artifacts. Use the validation gates that the selected Loop and repository
actually provide.

<a id="deferred-and-removed"></a>

## 5. Deferred and removed surfaces

This table is the exhaustive legacy-surface ledger. “Removed” means no compatibility alias, route,
loader, or hidden fallback ships in v0.3.

| v0.2.15 surface                                                                                | Status in v0.3                      | Successor or workaround                                                                                  |
| ---------------------------------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `tasks run` basic delivery                                                                     | Replaced                            | Bundled `software-delivery` Loop.                                                                        |
| `--parallel-tasks` dependency waves                                                            | Deferred                            | Run serially; a custom Loop may model explicit safe fan-out.                                             |
| `--multiple` / `--parallel` worktree isolation                                                 | Deferred                            | Create explicit worktrees/workspaces and start separate runs.                                            |
| Tasks-run five-step wizard                                                                     | Removed                             | Supply explicit `loop run` flags and inspect `--dry-run -o json`.                                        |
| TUI cockpit, `runs attach`, and `--attach ui`                                                  | Removed                             | Use the web run page, structured CLI, HTTP/UDS, and SSE.                                                 |
| `tasks validate`                                                                               | Removed                             | No substitute. `loop validate` is a different invariant.                                                 |
| `reviews fetch/list/show/fix/watch` verb family                                                | Removed as commands                 | `review-and-fix` is the only bundled review journey; inspect its `reviews-NNN/` files and run status.    |
| External review fetch/resolve, CodeRabbit/nitpicks, provider provenance, PR polling, auto-push | Removed                             | Reviews are authored by a Compozy reviewer agent. Push manually or from an outer Loop after review.      |
| Legacy review-provider extension abstraction and hooks                                         | Removed                             | No v0.3 external review-provider API.                                                                    |
| Recovery agent and recovery flags/config                                                       | Removed                             | Use Loop retry/no-progress/terminal outcomes and an explicit rerun or operator intervention.             |
| Cost-aware task budgets                                                                        | Deferred                            | Loop token/wall-clock budgets exist, but no legacy cost-aware per-task compatibility mapping exists.     |
| Agent-per-task rules                                                                           | Deferred                            | Use imported task `runtime` frontmatter or `--runtime id=<task>:...`; agent identity remains Loop-owned. |
| External-CLI skill/setup installer                                                             | Removed                             | Use daemon-managed `skill`, `agent`, and extension-resource surfaces.                                    |
| Reusable-agent `setup` assets                                                                  | Removed                             | Re-author runtime agent definitions through `compozy agent`; do not copy legacy setup state.             |
| `compozy ext` command and old manifest API                                                     | Replaced incompatibly               | Port to the v0.3 manifest/SDK and manage with `compozy extension`.                                       |
| `compozy migrate` XML converter                                                                | Legacy-only                         | Run v0.2.15 before upgrading when XML-era artifacts exist.                                               |
| `compozy migrate config`                                                                       | Deferred to Task 14                 | Use the manual matrix and clean-state procedure in this guide.                                           |
| `sync` / `archive`                                                                             | Removed                             | Runtime owners ingest their own resources; archive task folders manually when safe.                      |
| `runs watch` / `runs purge`                                                                    | Removed                             | Use Loop/session-specific lifecycle surfaces; legacy run history restarts cleanly.                       |
| `exec` ad-hoc runner                                                                           | Replaced by explicit sessions       | Use `session new` plus `session prompt` and lifecycle commands.                                          |
| `/` legacy workflow dashboard                                                                  | Replaced                            | The v0.3 root is the operating-system home.                                                              |
| `/memory` and `/memory/:slug`                                                                  | Replaced incompatibly               | Use `/knowledge` and `/settings/memory`; old page URLs do not redirect.                                  |
| `/reviews`, `/reviews/:slug/:round`, `/reviews/:slug/:round/:issueId`                          | Removed                             | Use `/loops/review-and-fix`, `/loop-runs/:runId`, and on-disk review artifacts.                          |
| `/runs` and `/runs/:runId`                                                                     | Replaced incompatibly               | Use `/loop-runs`, `/loop-runs/:runId`, `/session/:id`, or task-run routes according to owner.            |
| `/workflows`                                                                                   | Replaced                            | `/loops`.                                                                                                |
| `/workflows/:slug/spec`                                                                        | Replaced                            | `/loops/:name` and `/loops/:name/editor`.                                                                |
| `/workflows/:slug/tasks` and `/workflows/:slug/tasks/:taskId`                                  | Replaced incompatibly               | `/tasks` and `/tasks/:id`; Loop definitions and durable tasks are separate resources.                    |
| Public Go facade `compozy.go` and `pkg/compozy/{events,runs}`                                  | Removed                             | Pin v0.2.x for library use; v0.3 is an executable/runtime product.                                       |
| Public Go extension SDK `sdk/extension`                                                        | Removed as public compatibility API | Port against the private, version-matched `sdk/go` workspace in this repository.                         |
| Public npm `@compozy/extension-sdk@0.1.x`                                                      | Frozen, no v0.3 registry successor  | Pin 0.1.x for old extensions; current development uses the private `sdk/typescript` workspace.           |
| Public npm `@compozy/create-extension@0.1.x`                                                   | Frozen, no v0.3 registry successor  | Pin 0.1.x for old scaffolds; current development uses the private `sdk/create-extension` workspace.      |
| `cy-capture-decisions` extension and skill                                                     | Removed                             | No bundled v0.3 successor; preserve its decision files as repository artifacts if still useful.          |
| `cy-idea-factory` extension and skill                                                          | Removed                             | No bundled v0.3 successor. Recreate the process outside the runtime if required.                         |
| `cy-qa-workflow` extension                                                                     | Removed                             | Use the repository's current QA skills/process; no compatibility extension is enrolled.                  |
| Legacy `compozy` bundled skill                                                                 | Retired                             | Use the new official runtime guide plus the nine dev-cycle skills.                                       |
| Nine retained process skills                                                                   | Moved                               | Bundled resources of the `dev-cycle` extension.                                                          |
| Legacy daemon DB, run artifacts, sockets, locks, logs, and state directories                   | Not migrated                        | Start with clean v0.3 state; retain backups only for forensic reference.                                 |

The web route mapping is a semantic mapping, not a redirect promise. v0.3 intentionally ships no
legacy route aliases.

### Legacy README anchor disposition

The v0.2 README accumulated inbound links from package registries, search results, and external
articles. The v0.3 front door keeps each semantic successor at the matching heading. Removed
pipeline-era sections have no empty heading stub; their disposition is explicit here.

| v0.2 heading                  | Inbound anchor                | v0.3 disposition                                                                           |
| ----------------------------- | ----------------------------- | ------------------------------------------------------------------------------------------ |
| Highlights                    | `#-highlights`                | Preserved at README **Highlights**.                                                        |
| Installation                  | `#-installation`              | Preserved at README **Installation**.                                                      |
| Homebrew                      | `#homebrew`                   | **No successor during beta.** Homebrew remains on deprecated v0.2 until v0.3.0 stable.     |
| NPM                           | `#npm`                        | Preserved at README **NPM**, now `@compozy/cli@beta`.                                      |
| Go                            | `#go`                         | Preserved at README **Go**, with the explicit v0.3 beta version.                           |
| From Source                   | `#from-source`                | Preserved at README **From Source**, using the root build target.                          |
| How It Works                  | `#-how-it-works`              | Preserved at README **How It Works**.                                                      |
| Daemon Runtime Model          | `#daemon-runtime-model`       | Preserved at the daemon-owned runtime explanation.                                         |
| Task Schema v2                | `#task-schema-v2`             | Preserved at the task-to-Loop migration explanation.                                       |
| Config Files                  | `#-config-files`              | Preserved at README **Config Files**.                                                      |
| Reusable Agents               | `#reusable-agents`            | Preserved at README **Reusable Agents**.                                                   |
| Extensions                    | `#-extensions`                | Preserved at README **Extensions**.                                                        |
| SDK support                   | `#sdk-support`                | Preserved at the v0.3 SDK boundary.                                                        |
| Extension CLI                 | `#extension-cli`              | Preserved at the current extension commands.                                               |
| Learn more                    | `#learn-more`                 | Preserved at the extension documentation links.                                            |
| Ad Hoc Exec                   | `#-ad-hoc-exec`               | **No successor.** Use explicit durable sessions; see the removed-surface ledger above.     |
| Quick Start                   | `#-quick-start`               | Preserved at the v0.3 first-session flow.                                                  |
| 1. Install skills             | `#1-install-skills`           | **No successor as a numbered pipeline step.** Skills are daemon-managed resources in v0.3. |
| 2. (Optional) Create an Issue | `#2-optional-create-an-issue` | **No successor.** Issue creation is outside the v0.3 runtime quick start.                  |
| 3. Create a PRD               | `#3-create-a-prd`             | **No successor in the README.** The retained dev-cycle skill owns this optional process.   |
| 4. Create a TechSpec          | `#4-create-a-techspec`        | **No successor in the README.** The retained dev-cycle skill owns this optional process.   |
| 5. Break down into tasks      | `#5-break-down-into-tasks`    | **No successor in the README.** Durable tasks are created through current task surfaces.   |
| 6. Execute tasks              | `#6-execute-tasks`            | **No successor.** The v0.2 pipeline runner is replaced by Loops.                           |
| 7. Review                     | `#7-review`                   | **No successor.** Provider-backed review commands were removed.                            |
| 8. Fix review issues          | `#8-fix-review-issues`        | **No successor.** Use the bundled `review-and-fix` Loop.                                   |
| 9. Iterate and ship           | `#9-iterate-and-ship`         | **No successor.** Shipping policy belongs to each repository.                              |
| Skills                        | `#-skills`                    | Preserved at README **Skills**.                                                            |
| Workflow Memory               | `#-workflow-memory`           | Preserved at the current scoped-memory explanation.                                        |
| Supported Agents              | `#-supported-agents`          | Preserved at the runtime/provider explanation.                                             |
| CLI Reference                 | `#-cli-reference`             | Preserved at the generated CLI reference link.                                             |
| Development                   | `#-development`               | Preserved at README **Development**.                                                       |
| Star History                  | `#star-history`               | Preserved at the repository widget.                                                        |
| Contributing                  | `#-contributing`              | Preserved at README **Contributing**.                                                      |
| License                       | `#-license`                   | Preserved at README **License**.                                                           |

<a id="license"></a>

## 6. License metadata correction

Compozy remained MIT-licensed. Some old distribution metadata incorrectly stamped artifacts as
`BSL-1.1`; v0.3 corrects that metadata to match the repository's MIT license. This is a distribution
metadata correction, **not a relicense**.

<a id="go-library"></a>

## 7. Go library and SDK close-out

v0.3 is distributed as the CompozyOS binary and runtime, not as a compatibility release of the old
public Go library. Code importing the root facade, `pkg/compozy/events`, `pkg/compozy/runs`, or the old
Go extension SDK will not compile against v0.3. Pin the final v0.2.x module while you maintain that
integration, or redesign it against v0.3 CLI/HTTP/UDS/tool/extension contracts.

The public npm extension packages remain on their v0.1.x line. They are not overwritten with the
API-incompatible v0.3 workspaces. Repository contributors may use `sdk/go`, `sdk/typescript`, and
`sdk/create-extension` at the exact repository version; those workspaces are private and are not a
registry compatibility promise.

<a id="domain-and-clean-state"></a>

## 8. Domain, channels, and clean state

`https://compozy.com` serves the v0.3 site from the single cut. `agh.network` retires without a
redirect layer. Legacy source and release collateral remain on the `legacy/v0.2` branch.

During the beta window:

- use `@compozy/cli@beta`, the hosted installer with an explicit v0.3 beta, or versioned
  `go install`;
- do not use Homebrew for v0.3 — the formula remains on the deprecated v0.2 stable line until the
  post-beta 0.3.0 stable release;
- a beta self-update follows newer v0.3 prereleases, not npm `latest` or the v0.2 stable line.

Start runtime history clean. Preserve committable task artifacts under `.compozy/tasks/`, translate
configuration manually, and do not carry daemon databases or legacy extension/setup directories
into the new runtime. The task schema remains useful; runtime state does not.

For Git, keep authored task/config artifacts according to your repository policy and ignore runtime
state inside the shared directory. At minimum, review rules for:

```gitignore
.compozy/compozy.db
.compozy/workspace.toml
.compozy/daemon.sock
.compozy/daemon.lock
.compozy/logs/
.compozy/cache/
```

Do not ignore the entire `.compozy/` directory when the repository commits `.compozy/tasks/`, Loop
definitions, skills, agents, or workspace configuration. After backup and manual translation, run a
dry delivery plan first, inspect the reported input origins/runtime provenance, then start the real
run and follow the final-line `/loop-runs/<id>` URL.
