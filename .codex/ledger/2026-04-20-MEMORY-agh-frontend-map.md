Goal (incl. success criteria):

- Map the AGH frontend stack in `/Users/pedronauck/dev/compozy/agh`, focusing on workspace/package structure, web toolchain, shared UI organization, and daemon embedding/serving behavior.
- Success = concise report with package map, exact frontend toolchain/deps, serving pattern with file refs, and alignment notes for looper.

Constraints/Assumptions:

- Read-only against `/Users/pedronauck/dev/compozy/agh`; no edits there.
- Must respect repo instructions in `agh/AGENTS.md` and `agh/web/AGENTS.md`.
- Current workspace instructions require no destructive git commands.

Key decisions:

- Treat `web/` as the daemon-served SPA and `packages/site/` as the separate docs/marketing Next app.
- Treat `packages/ui/` as the shared shadcn-derived UI kit consumed by `web/` via `@agh/ui`.

State:

- Collected manifests, Vite/Vitest configs, Tailwind/shadcn config, UI token file, and daemon static-asset serving code.

Done:

- Identified the default build path: `make build` runs `web` build before Go build.
- Identified embedded asset path: `web/embed.go` + `internal/api/httpapi/static.go`.

Now:

- Summarize the architecture with exact file references and keep it concise.

Next:

- Provide the report and note any structural patterns looper should mirror.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/agh/AGENTS.md`
- `/Users/pedronauck/Dev/compozy/agh/web/AGENTS.md`
- `/Users/pedronauck/Dev/compozy/agh/package.json`
- `/Users/pedronauck/Dev/compozy/agh/web/package.json`
- `/Users/pedronauck/Dev/compozy/agh/packages/ui/package.json`
- `/Users/pedronauck/Dev/compozy/agh/web/vite.config.ts`
- `/Users/pedronauck/Dev/compozy/agh/web/vitest.config.ts`
- `/Users/pedronauck/Dev/compozy/agh/packages/ui/vitest.config.ts`
- `/Users/pedronauck/Dev/compozy/agh/web/embed.go`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/static.go`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/server.go`
- `/Users/pedronauck/Dev/compozy/agh/Makefile`
