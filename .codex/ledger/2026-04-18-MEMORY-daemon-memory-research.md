Goal (incl. success criteria):

- Corrigir a causa raiz do problema de CPU/memória que estava impedindo finalizar `.compozy/tasks/daemon/task_15.md`.
- Sucesso = remover a recursão de auto-start do daemon sob `go test`, estabilizar os testes daemon-backed de CLI e fechar com `make verify` verde sem workarounds.

Constraints/Assumptions:

- Seguir `AGENTS.md`, `CLAUDE.md` e o task `15`; sem comandos git destrutivos e sem tocar em arquivos não relacionados.
- Skills ativas nesta correção: `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Evitar qualquer reprodução que possa relançar processos fora de controle; verificações rodadas com `GOMAXPROCS=2` e `GOFLAGS='-p=1'`.

Key decisions:

- Rejeitar explicitamente relançamento de binário `*.test` no auto-start do daemon em vez de mascarar o problema.
- Dar aos testes de `exec`/`fix-reviews` um bootstrap in-process controlado do daemon, sem subprocesso externo.
- Serializar overrides globais de bootstrap/observers em `internal/cli` para eliminar a corrida sob `-race`.

State:

- Completed.

Done:

- Identifiquei e corrigi a causa raiz em `internal/cli/daemon_commands.go`: o launcher agora recusa relançar binário de teste Go (`*.test`) e expõe erro explícito em vez de recursão de processo.
- Adicionei teste de regressão em `internal/cli/daemon_commands_test.go` para garantir a rejeição de binário de teste.
- Criei bootstrap in-process seguro para testes daemon-backed em `internal/cli/daemon_exec_test_helpers_test.go`.
- Corrigi bugs reais expostos pela migração daemon-backed em `internal/core/run/exec/exec.go`, `internal/daemon/run_manager.go`, `internal/cli/reviews_exec_daemon.go` e `internal/cli/run.go`:
  - diferenciação correta entre run novo e resume persistido;
  - propagação de `agent` e flags explícitas de runtime pelo daemon;
  - fallback robusto para terminal event/snapshot;
  - preservação da UX de erro de reusable agents através da fronteira do daemon.
- Corrigi isolamento de testes em `internal/cli/daemon_commands_test.go` com serialização dos hooks globais de bootstrap/observer, resolvendo a última falha sob `-race`.
- Verificações concluídas com sucesso:
  - `GOMAXPROCS=2 GOFLAGS='-p=1' go test ./internal/cli -race -count=1`
  - `GOMAXPROCS=2 GOFLAGS='-p=1' make verify`

Now:

- Preparar handoff curto com causa raiz, correção aplicada e evidência de verificação.

Next:

- Nenhum passo técnico pendente para esta correção.

Open questions (UNCONFIRMED if needed):

- Nenhuma aberta para esta correção.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-daemon-memory-research.md`
- `internal/cli/{daemon_commands.go,daemon_commands_test.go,daemon_exec_test_helpers_test.go,reviews_exec_daemon.go,run.go,root_command_execution_test.go,agents_commands_test.go,reusable_agents_doc_examples_test.go,commands.go}`
- `internal/core/{fetch.go,run/exec/exec.go,run/exec/exec_test.go}`
- `internal/daemon/{run_manager.go,run_manager_test.go}`
- Commands: `rg`, `sed`, `gofmt`, `go test`, `make verify`, `git status --short`
