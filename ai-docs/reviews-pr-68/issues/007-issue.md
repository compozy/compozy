# Issue 7 - Review Thread Comment

**File:** `pkg/compozy/runs/tail.go:122`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** O follower de `Tail()` agora usa `MustExist: true`, eliminando a goroutine que poderia ficar esperando indefinidamente por um arquivo inexistente quando o arquivo live não existe mais entre o snapshot e o follow.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`Does nxadm/tail TailFile with MustExist: false block waiting for file creation or return an error?`

💡 Result:

`MustExist: false` does **not** make `TailFile` return an error when the file is missing. `TailFile(...)` returns `(*Tail, nil)` immediately and starts a goroutine; that goroutine then calls `reopen()`, which—on `os.IsNotExist(err)`—logs “Waiting for … to appear…” and **blocks in `watcher.BlockUntilExists(...)` until the file is created** (or the tail is stopped / the watcher returns an error). [1]

**So: it waits (blocks) for file creation; it only errors if creation detection/open ultimately fails.** [1]

[1] https://raw.githubusercontent.com/nxadm/tail/v1.4.11/tail.go

---



</details>

**Consider the blocking behavior with `MustExist: false` for missing event files.**

With `MustExist: false`, `TailFile` returns immediately but spawns a goroutine that blocks indefinitely in `watcher.BlockUntilExists(...)` if the file doesn't exist. If a run completes before `Tail` is called and the events file is removed or never created, this will block until the goroutine's context is cancelled. Verify this is intentional and ensure context cancellation is properly connected to the returned `Tail` object. If this is not desired behavior, consider using `MustExist: true` to surface an error for missing files or add explicit handling for non-existent completed runs.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/runs/tail.go` around lines 113 - 122, The openTailFollower
function currently sets tailpkg.Config{MustExist: false}, which causes TailFile
to spawn a goroutine that can block indefinitely if the events file is missing;
either make missing files surface as an error by changing MustExist to true in
Run.openTailFollower (so TailFile returns an error for non-existent event
files), or refactor openTailFollower to accept a context parameter and set
tailpkg.Config.Context to that context so the spawned goroutine is cancelable;
update references to openTailFollower accordingly and ensure you modify the
Config.MustExist or Config.Context fields (and call sites) to prevent unbounded
blocking.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:2974b5e6-6986-4720-8bc5-83199c8c0f48 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9ut`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9ut
```

---
*Generated from PR review - CodeRabbit AI*
