Goal (incl. success criteria):

- Propor uma estratégia melhor de desenvolvimento local para `daemon + web` com hot reload no frontend, reduzindo a necessidade de `make build && ./bin/compozy daemon stop && ./bin/compozy daemon start` a cada mudança em `web/`.
- Success means: entender o fluxo atual de build/serving, identificar opções viáveis com trade-offs claros, recomendar um caminho, e capturar perguntas abertas para um eventual design/implementação.

Constraints/Assumptions:

- O usuário pediu sugestão/brainstorming, não implementação imediata.
- A skill `brainstorming` foi acionada; não devo implementar nada antes de apresentar abordagens e obter aprovação.
- O daemon hoje serve assets embedded do frontend; isso favorece produção, mas piora DX se for o único modo local.
- Não usar busca web para código local.

Key decisions:

- Tratar o problema como separação entre `modo dev` e `modo embedded/prod`, não como ajuste no pipeline de produção.
- Investigar primeiro os pontos reais de acoplamento: `Makefile`, `web/package.json`, `web/vite.config.ts`, `internal/api/httpapi`, e ledgers existentes sobre `daemon-web-ui`.
- Prioridade de DX confirmada pelo usuário: preservar experiência de URL única também no desenvolvimento local.
- Manter `make dev` como default seguro com `HOME` isolado e expor `make dev-global` como variante explícita que reaproveita o mesmo fluxo Turbo+Vite apontando para o `HOME` real do usuário.
- Simplificar a orquestração local removendo o workspace `dev/daemon`: `make dev` e `make dev-global` passam a usar `./bin/compozy` diretamente e deixam o hot reload apenas no frontend via Vite.

State:

- Design aprovado e documentado.
- Implementação concluída e verificada.

Done:

- Li `.agents/skills/brainstorming/SKILL.md`.
- Li ledgers relevantes:
  - `.codex/ledger/2026-04-20-MEMORY-daemon-web-ui.md`
  - `.codex/ledger/2026-04-20-MEMORY-web-ui-architecture.md`
- Confirmei no repositório:
  - `web/package.json` tem `dev: "bun run codegen && vite --port 3000"`.
  - `web/vite.config.ts` já faz proxy de `/api` para `http://localhost:2123`.
  - `web/embed.go` embute `web/dist`.
  - `internal/api/httpapi/static.go` serve assets embedded e fallback SPA.
  - `internal/api/httpapi/server.go` sempre carrega `newStaticFS()` no startup.
  - `Makefile` hoje só expõe `frontend-build`/`build`/`verify`; não há alvo dedicado de DX para hot reload.
- Usuário confirmou que quer preservar experiência de URL única no browser durante desenvolvimento local.
- Salvei o design aprovado em `docs/plans/2026-04-21-hot-reload-dev-design.md`.
- Implementei a base do modo dev por proxy:
  - `internal/api/httpapi/dev_proxy.go`
  - `internal/api/httpapi/server.go` agora escolhe entre bundle embedded e proxy dev
  - `internal/daemon/host.go` propaga `WebDevProxyTarget`
  - `internal/cli/daemon.go` ganhou `COMPOZY_WEB_DEV_PROXY` + flag `--web-dev-proxy`
- Implementei a orquestração local via Turbo:
  - `dev/daemon/package.json`
  - `scripts/dev-daemon.sh`
  - `package.json` workspaces/scripts
  - `web/vite.config.ts`
  - `Makefile` alvo `dev`
- Simplifiquei a orquestração local para binário compilado + Vite:
  - removi `dev/daemon/package.json`
  - removi `scripts/dev-daemon.sh`
  - adicionei `scripts/dev-web-proxy.sh`
  - `Makefile` agora chama `./bin/compozy` diretamente
  - `package.json` não precisa mais do workspace `dev/*` nem scripts de daemon dev
- Adicionei/ajustei testes:
  - `internal/api/httpapi/dev_proxy_test.go`
  - `internal/cli/daemon_commands_test.go`
  - `test/frontend-workspace-config.test.ts`
- Adicionei a variante global de DX:
  - `package.json` ganhou `dev:global`
  - `Makefile` ganhou `dev-global`
  - `test/frontend-workspace-config.test.ts` agora valida o novo script raiz
- Validei o novo entrypoint `dev-global`:
  - `bunx vitest run --config vitest.config.ts test/frontend-workspace-config.test.ts`
  - `make verify`
  - ambos passaram
- Regenerado `bun.lock` para refletir a remoção do workspace `dev/*`; isso era necessário para `bun ci` voltar a aceitar o lockfile congelado.
- Atualizei `docs/plans/2026-04-21-hot-reload-dev-design.md` para refletir a implementação final baseada em `./bin/compozy`.
- Reexecutei `make verify` após o sync documental; passou novamente.
- Corrigi falhas latentes expostas pela suíte completa:
  - helper de teste do daemon agora serializa o override de `HOME`
  - testes de `internal/core/extension` agora isolam `HOME`
  - ajustes em testes/mapeadores para compatibilidade com `json.RawMessage` e limites de path no macOS
- Executei `make verify` com sucesso.

Now:

- Nenhuma ação pendente; pronto para handoff.

Next:

- Se o usuário quiser, o próximo passo natural é adicionar uma variante equivalente para hot reload do backend Go.
- Se o usuário quiser, o próximo passo natural é adicionar uma variante equivalente para hot reload do backend Go.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: se o time aceita dois processos locais por baixo dos panos (`daemon` + `vite`) desde que o browser continue usando só a URL do daemon.
- UNCONFIRMED: se querem HMR completo com websocket do Vite ou apenas live reload com rebuild incremental.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-hot-reload-dev.md`
- `.codex/ledger/2026-04-20-MEMORY-daemon-web-ui.md`
- `.codex/ledger/2026-04-20-MEMORY-web-ui-architecture.md`
- `Makefile`
- `package.json`
- `bun.lock`
- `scripts/dev-web-proxy.sh`
- `test/frontend-workspace-config.test.ts`
- `web/package.json`
- `web/vite.config.ts`
- `web/embed.go`
- `internal/api/httpapi/{server.go,static.go,browser_middleware.go}`
- Commands used:
  - `rg --files`
  - `rg -n "..."`
  - `sed -n`
  - `bunx vitest run --config vitest.config.ts test/frontend-workspace-config.test.ts`
  - `make verify`
