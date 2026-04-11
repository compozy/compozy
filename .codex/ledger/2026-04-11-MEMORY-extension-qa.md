Goal (incl. success criteria):

- Atuar como QA/usuário da feature de extensões introduzida a partir de `.compozy/tasks/ext`, exercendo fluxos reais de uso, registrando issues em `.compozy/tasks/ext/qa/review_<num>.md` e corrigindo causas raiz na mesma sessão.
- Sucesso requer evidência de testes amplos de uso, correções para bugs encontrados, e verificação final com `make verify`.

Constraints/Assumptions:

- Seguir `AGENTS.md`, `CLAUDE.md` e skills obrigatórios para QA, bugfix, Go e verificação final.
- Não tocar nem reverter alterações sujas não relacionadas, em especial o rename local entre `.compozy/tasks/extensibility` e `.compozy/tasks/ext`.
- O commit de referência é `a1aa7813f4ab`; a validação será feita no estado atual do workspace.

Key decisions:

- Usar os ledgers de extensões existentes como contexto de implementação e focar em testes end-to-end e fluxos de operador/autor de extensão.
- Registrar cada bug confirmado antes de corrigi-lo, mantendo rastreabilidade no diretório `.compozy/tasks/ext/qa/`.

State:

- Concluído após QA manual ampla, correções de causa raiz e `make verify` limpo.

Done:

- Li as instruções do repositório, os skills obrigatórios, o diff do commit `a1aa7813f4ab`, os ledgers relevantes de extensões e os documentos `_techspec.md`, `_tasks.md`, `_protocol.md` e `task_15.md`.
- Executei QA manual como usuário/operador cobrindo scaffold TS, scaffold Go, `ext install/enable/list/inspect`, `exec --extensions --dry-run --persist`, auditoria de hooks e execução cross-workspace.
- Registrei os bugs confirmados em `.compozy/tasks/ext/qa/review_001.md`.
- Corrigi quatro causas raiz:
  - artefato empacotado de `@compozy/create-extension` sem binário CLI funcional;
  - ausência de dispatch de hooks de prompt/run em `exec --extensions`;
  - resolução incorreta do diretório de trabalho de extensões instaladas;
  - fallback quebrado do scaffold Go para `sdk/extension`.
- Adicionei/atualizei testes em `sdk/create-extension`, `internal/core/run/exec`, `internal/core/extension` e `internal/core/subprocess`.
- Revalidei os fluxos manualmente após as correções.
- Rodei `make verify` com sucesso: fmt, lint, 1688 testes e build passaram.

Now:

- Nenhum trabalho restante.

Next:

- Opcional: remover este ledger em um follow-up se não houver continuidade nesta sessão.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: se o estado atual do workspace já inclui mudanças pós-commit que impactam os fluxos de QA além do rename do task.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-11-MEMORY-extension-qa.md`
- `.compozy/tasks/ext/qa/review_001.md`
- `.compozy/tasks/ext/_techspec.md`
- `.compozy/tasks/ext/_tasks.md`
- `.compozy/tasks/ext/_protocol.md`
- `.compozy/tasks/ext/task_15.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-foundation.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-bootstrap.md`
- `.codex/ledger/2026-04-10-MEMORY-ext-cli-state.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-lifecycle.md`
- `.codex/ledger/2026-04-10-MEMORY-hook-dispatches.md`
- `internal/core/run/exec/exec.go`
- `internal/core/run/exec/hooks.go`
- `internal/core/run/exec/exec_test.go`
- `internal/core/extension/manager_spawn.go`
- `internal/core/extension/manager_test.go`
- `internal/core/subprocess/process.go`
- `internal/core/subprocess/process_unix_test.go`
- `sdk/create-extension/{package.json,src/index.ts,README.md,test/create-extension.test.ts}`
- `sdk/create-extension/bin/create-extension.ts`
- `sdk/create-extension/scripts/copy-templates.mjs`
- Commands: `git show --stat a1aa7813f4ab`, `rg`, `sed`, `go test ./internal/core/extension ./internal/core/run/exec ./internal/core/subprocess -count=1`, `npx vitest run sdk/create-extension/test/create-extension.test.ts`, `make verify`
