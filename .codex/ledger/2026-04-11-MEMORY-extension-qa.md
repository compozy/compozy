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

- Em andamento.

Done:

- Li as instruções do repositório, os skills obrigatórios, o diff do commit `a1aa7813f4ab`, os ledgers relevantes de extensões e os documentos `_techspec.md`, `_tasks.md`, `_protocol.md` e `task_15.md`.

Now:

- Mapear fluxos prioritários de uso e executar reproduções reais.

Next:

- Rodar comandos de uso da CLI `compozy ext`, scaffold/build de templates TS/Go, e fluxos de runtime com extensões habilitadas.
- Registrar issues confirmadas em `.compozy/tasks/ext/qa/review_<num>.md` e corrigir.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: se o estado atual do workspace já inclui mudanças pós-commit que impactam os fluxos de QA além do rename do task.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-11-MEMORY-extension-qa.md`
- `.compozy/tasks/ext/_techspec.md`
- `.compozy/tasks/ext/_tasks.md`
- `.compozy/tasks/ext/_protocol.md`
- `.compozy/tasks/ext/task_15.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-foundation.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-bootstrap.md`
- `.codex/ledger/2026-04-10-MEMORY-ext-cli-state.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-lifecycle.md`
- `.codex/ledger/2026-04-10-MEMORY-hook-dispatches.md`
- Commands: `git show --stat a1aa7813f4ab`, `rg`, `sed`
