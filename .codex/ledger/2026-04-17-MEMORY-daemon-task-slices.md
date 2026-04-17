Goal (incl. success criteria):

- Mapear os slices técnicos do feature `daemon` no codebase atual do Compozy, com foco em: árvore de comandos do CLI e caminho `ensureDaemon`, implicações de TUI attach/watch, seams de migração de `pkg/compozy/runs`, e padrões de `validate-tasks`/parser/testes que afetam criação de tasks e rollout.
- Success means: responder em pt-BR com arquivos atuais a tocar/criar, riscos de acoplamento, e dependências naturais por slice, sem propor tasks finais ainda.

Constraints/Assumptions:

- Leitura mínima, foco em arquivos diretamente relevantes.
- Não alterar código nesta exploração.
- Não usar comandos git destrutivos.
- Manter a resposta em pt-BR.

Key decisions:

- Tratar a árvore CLI atual como ponto de inserção do futuro client path do daemon.
- Tratar `pkg/compozy/runs` como seam de compatibilidade/migração, não como API congelada do daemon.
- Tratar `validate-tasks` e o parser de tasks como superfície de rollout sensível, porque já alimentam preflight, migração e forms.

State:

- Explorando e sintetizando.

Done:

- Lido `AGENTS.md` e orientações do repo.
- Lidos os arquivos de arquitetura/design do daemon e os pacotes principais ligados a CLI, TUI, runs e tasks.
- Identificados os pontos de acoplamento mais prováveis para a decomposição.

Now:

- Consolidar a síntese final por slice, citando arquivos e riscos.

Next:

- Se necessário depois, converter a leitura em backlog/TechSpec, mas ainda não nesta resposta.

Open questions (UNCONFIRMED if needed):

- `ensureDaemon` ainda não existe no codebase; o ponto exato de integração do client path permanece a ser definido.
- O formato final de attach/watch do daemon ainda depende do contrato de transporte que vier na próxima etapa.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-daemon-task-slices.md`
- `internal/cli/root.go`
- `internal/cli/commands.go`
- `internal/cli/state.go`
- `internal/core/run/ui/{model.go,view.go,update.go,summary.go,layout.go,types.go}`
- `pkg/compozy/runs/{run.go,summary.go,watch.go,tail.go,replay.go,status.go,layout/layout.go}`
- `internal/core/tasks/{parser.go,validate.go,walker.go,store.go}`
- `internal/cli/{validate_tasks.go,task_runtime_form.go,migrate_command_test.go,validate_tasks_test.go,root_command_execution_test.go}`
- `internal/core/migration/migrate.go`
- `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- `internal/core/kernel/{handlers.go,commands/run_start.go}`
