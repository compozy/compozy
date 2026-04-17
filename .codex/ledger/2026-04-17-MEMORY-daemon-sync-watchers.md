Goal (incl. success criteria):

- Enriquecer as tasks restantes do feature daemon ligadas a sync/archive/watchers (`task_07`, `task_08`, `task_09`) com arquivos reais, dependências e testes prioritários.
- Success means: resposta em pt-BR, estruturada por task, sem editar código de produto.

Constraints/Assumptions:

- Não editar arquivos de produção.
- Responder com foco em arquivos reais do codebase e metadados legacy relevantes (`_meta.md`, `_tasks.md`).
- Não usar comandos git destrutivos.

Key decisions:

- Tratar `internal/core/sync.go`, `internal/core/archive.go`, `internal/core/tasks/*`, `internal/core/reviews/*`, `pkg/compozy/runs/*`, `internal/core/run/*` como a base factual.
- Usar `.compozy/tasks/daemon/_techspec.md`, `_tasks.md`, `task_02.md`, `task_04.md` e artefatos legados para inferir dependências e cobertura.

State:

- Contexto suficiente para sintetizar a resposta final.

Done:

- Lidos `_techspec.md`, `_tasks.md`, ledgers anteriores e tasks `task_01`, `task_02`, `task_04`.
- Inspecionados `internal/core/sync.go`, `internal/core/archive.go`, `internal/core/tasks/store.go`, `internal/core/reviews/{store.go,parser.go}`, `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`.
- Inspecionados `pkg/compozy/runs/{run.go,summary.go,watch.go,tail.go,status.go,layout/layout.go}` e `internal/core/run/{ui/model.go,executor/execution.go,journal/journal.go,event_stream.go}`.
- Levantados testes existentes em `internal/core/*` e `pkg/compozy/runs/*`.

Now:

- Montar a lista final por task com arquivos relevantes, dependentes, requisitos concretos e testes mais importantes.

Next:

- Se o usuário pedir, transformar isso em backlog detalhado por subtarefa.

Open questions (UNCONFIRMED if needed):

- Nenhuma bloqueante.

Working set (files/ids/commands):

- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_02.md`
- `.compozy/tasks/daemon/task_04.md`
- `internal/core/sync.go`
- `internal/core/archive.go`
- `internal/core/tasks/store.go`
- `internal/core/reviews/store.go`
- `internal/core/reviews/parser.go`
- `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- `pkg/compozy/runs/{run.go,summary.go,watch.go,tail.go,status.go,layout/layout.go}`
- `internal/core/run/{ui/model.go,executor/execution.go,journal/journal.go,event_stream.go}`
