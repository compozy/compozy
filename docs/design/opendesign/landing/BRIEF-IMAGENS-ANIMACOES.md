# Landing compozy.com — brief de imagens e animações a gerar

Revisão 2026-09-10. Todas as imagens e as duas animações da landing (`landing/landing-page.html`) foram substituídas por placeholders e serão geradas de novo. Este arquivo é o brief de cada uma: onde entra, geometria do slot, o que precisa mostrar e o que evitar. Cada placeholder na página repete o resumo no próprio lugar; abra `landing-page.html?annotate` para ver todos os slots contornados e rotulados.

Regras que valem para tudo:

- **Paleta e luz do produto.** Fundo `--rail` / `--canvas` (#0c0b0b / #171615), superfícies quentes-escuras, hairlines brancas a 5–14 %, um único acento laranja `--accent` (#e8572a) usado como sinal, nunca como lavagem. Sem gradientes roxos, sem néon, sem glow espalhado.
- **Verdade de runtime.** Capturas mostram só o que o daemon realmente tem: rotas reais, contagens plausíveis, nenhum id inventado, nenhum controle que não exista. Ilustrações conceituais não desenham UI falsa.
- **Sem texto renderizado dentro de imagem gerada.** Rótulos, títulos e código ficam no HTML; imagem gerada com texto legível vira ruído e envelhece mal.
- **Geometria do slot é lei.** Gerar já na proporção do slot (16:10, 16:9, 21:9 ou painel) em vez de cortar depois. Exportar em `.webp`, largura mínima 2× do slot renderizado (hero e capturas ≥ 2400 px de largura; spots ≥ 1600 px).
- **Um estilo por família.** Capturas são capturas (o shell real, mesma escala, mesmas margens). Spots e artes conceituais compartilham uma linguagem entre si (mesma câmera, mesmo material, mesma luz). Não misturar renders 3D com flat ou com screenshot dentro da mesma família.
- **Destino.** Arquivos em `landing/assets/` com o nome indicado; os arquivos antigos ainda estão lá como referência do que não funcionou e podem ser apagados quando o novo set entrar.

---

## 1. Hero — poster e clipes de demo

| Campo | Valor |
| --- | --- |
| Onde | Seção hero, `data-od-id="hero-demo-plate"` |
| Slot | `.plate` 16:10, ~1120 × 700 px renderizado; gerar ≥ 2400 × 1500 |
| Arquivo | `hero-poster.webp` (poster) + `demo-1.webm … demo-6.webm` (clipes) |
| Hoje | placeholder "Hero demo poster" |

O plate do hero tem seis abas (`Agent session`, `Loop editor`, `Scheduled jobs`, `Tasks inbox`, `Home`, `Event triggers`). Hoje um único poster serve as seis abas e o JS só faz pan/zoom nele. O objetivo é gravar **seis clipes curtos** em um lab semeado, com **um poster por clipe** (primeiro frame limpo), e o plate trocar clipe ao trocar de aba.

Cada clipe: 8–12 s, sem áudio, loop suave, 16:10, cursor visível só quando a ação precisa dele, sem cortes bruscos. Rotas e o que cada um mostra:

| Aba | Rota | O clipe mostra |
| --- | --- | --- |
| Agent session | `/agents/$name/sessions/$id` | abrir uma sessão, o trabalho streamando (linha de tool viva), uma pergunta de permissão sendo respondida |
| Loop editor | `/loops/$name/editor` | abrir um Loop bundled, fazer fork do grafo: contrato, nós, publicar |
| Scheduled jobs | `/jobs` | colocar um agente em agenda; a run aparecendo no histórico |
| Tasks inbox | `/tasks?mode=inbox` | aprovar, rejeitar ou re-tentar trabalho que espera decisão |
| Home | `/` | o que precisa de você e o que está trabalhando agora, numa tela |
| Event triggers | `/triggers` | trabalho começando a partir de um evento de sessão ou de um webhook assinado |

O poster de fallback (enquanto os clipes não existem) é a captura do desktop com as janelas Loops e Tasks abertas sobre o dock e a menubar — mesma cena do slot 14 abaixo, mas com margens cortadas para preencher o plate.

## 2. Use cases — cinco spots

Seção `use-cases`, cards `.uc`. Cinco ilustrações da mesma família visual: objetos abstratos do produto (dispositivo do daemon, listas, anéis, timelines) em material fosco escuro, uma fonte de luz, um toque de laranja como sinal. Nada de personagens, nada de UI desenhada, nada de texto.

| # | Slot (`data-od-id`) | Proporção | Arquivo | O que mostrar |
| --- | --- | --- | --- | --- |
| 2.1 | `use-case-d1` Implement | 16:9 | `spot-implement.webp` | o daemon ao lado de uma lista de tarefas flutuante; três linhas acendem em ordem (trabalho em dependência, sessão durável) |
| 2.2 | `use-case-d2` Review | 16:9 | `spot-review.webp` | anéis em órbita ao redor de um marcador de run, uma volta após a outra (revisor + corretor até a rodada sair limpa) |
| 2.3 | `use-case-o1` Briefing | 16:9 | `spot-briefing.webp` | um job agendado se abrindo em quatro passos: arquivar, notificar, webhook, resumo |
| 2.4 | `use-case-d3` Release | 21:9 | `spot-release.webp` | uma fila numerada de runs numa linha, a atual com um anel (checks de release como trabalho reivindicado) |
| 2.5 | `use-case-o3` Gate | 21:9 | `spot-gate.webp` | uma timeline de run pausada num portão, checks atrás dele, próximos passos esperando (aprovação humana) |

Os dois wide (Release e Gate) precisam de composição horizontal de verdade, não o quadrado esticado — foi o erro dos anteriores. O chão da ilustração é a cor do card (`--canvas-soft`, #1F1E1C), plano, sem vinheta: a imagem não tem moldura e precisa sumir no card; deixar o motivo principal na metade de cima.

## 3. Features — seis capturas do produto

Seção `features`, janelas `.win` 16:10 com o chrome (título + rota) desenhado em HTML; a captura preenche só o corpo da janela. Capturar em um lab semeado com dados reais (workspace `~/Dev/compozy` ou equivalente), escala 2×, tema escuro, sem cursor, sem dados sensíveis. Recortar o chrome nativo; a landing coloca o seu.

| # | Slot | Rota | Arquivo | O que a captura precisa conter |
| --- | --- | --- | --- | --- |
| 3.1 | Sessions | `/agents/$name/sessions/$id` | `feature-sessions.webp` | uma sessão com timeline real: arquivo editado, testes rodando, commit, sessão pausada; o diff ao lado |
| 3.2 | Knowledge (Memory) | `/knowledge` | `feature-memory.webp` | a base de conhecimento com arquivos Markdown reais e as superfícies que os leem |
| 3.3 | Tasks | `/tasks?mode=kanban` | `capture-tasks-window.webp` | o quadro kanban com grupos blocked / queued / done, donos e prioridades (hoje só existe a captura em lista) |
| 3.4 | Jobs (Automation) | `/jobs` | `feature-automation.webp` | um job com histórico de runs: o trace de uma run ao lado do gráfico de eventos por dia |
| 3.5 | Desktops (shell) | desktop inteiro | `hero-poster.webp` (reuso) | menubar, duas janelas em tile e o dock — a mesma cena que serve de poster do hero |
| 3.6 | Loop run | `/loop-runs/$runId` | `capture-loops-window.webp` | uma run real em `needs-approval`; a faixa "Needs you" é HTML e fica por cima, então deixar o terço inferior calmo |

Profiles, Gateway e Workspaces continuam como diagramas SVG inline (verdade de estado, três switches off) — não são imagens a gerar. Os fundos deles estão no item 5.

## 4. Artes conceituais — três painéis

Mesma família visual dos spots (item 2), mas em composição de painel, sem o degradê do card.

| # | Slot | Geometria | Arquivo | O que mostrar |
| --- | --- | --- | --- | --- |
| 4.1 | Extensions (`.ext__art`) | backdrop da seção, não painel: sangra pela borda direita da viewport sob as 5 últimas colunas (texto nas 7 primeiras), altura da seção, some por máscara antes da coluna de texto e nas bordas de cima e de baixo; gerar ~1200 × 1600 com o chão na cor da página (`--canvas`, #171615), plano, sem vinheta | `ext-cartridges.webp` | um pacote único encaixando nos registries do daemon: skills, hooks, tools, automation e extensions como módulos que se conectam a um mesmo bloco |
| 4.2 | Bridges (`.bridges__canvas`) | painel horizontal, ≥ 400 px de altura, ancorado à direita; gerar ~2000 × 900 | `bridges-inflow.webp` | mensagens fluindo de fora (Slack, Discord, Telegram, Google Chat) para o daemon; os logos são HTML por cima, a imagem só faz o fluxo; lado esquerdo e o terço inferior ficam livres para o conteúdo |
| 4.3 | Closer (`.cta__art`) | metade direita da seção final, full-bleed, ~66 % de largura × 112 % de altura; gerar ~2400 × 1400 | `closer-shell.webp` | o shell do CompozyOS em repouso — o desktop de longe, luz baixa; motivo à direita, esquerda escurecendo para o título |

## 5. Atmosferas — quatro texturas de fundo

Texturas de baixo contraste, sem motivo reconhecível, que ficam a 24–50 % de opacidade atrás de seções e diagramas. Gerar em ≥ 2000 px, quase monocromáticas sobre `--rail`, com no máximo um traço quente. Se lidas isoladas devem parecer "nada"; a página é quem dá o contexto.

| # | Onde | Arquivo | Caráter |
| --- | --- | --- | --- |
| 5.1 | hero (`.hero__wave`, 24 %, blend screen) | `hero-wave.webp` | uma onda larga e suave atravessando a direita do hero |
| 5.2 | pain + diagrama Workspaces (`.pain__atmo`, `.dg__atmo` traces) | `backdrop-traces.webp` | traços finos de trilhas, como rastros de eventos |
| 5.3 | loops + diagrama Profiles (`.loop__atmo`, `.dg__atmo` orbit) | `backdrop-orbit.webp` | arcos orbitais amplos, um ponto de luz |
| 5.4 | diagrama Gateway (`.dg__atmo` radar) | `backdrop-radar.webp` | varredura radial discreta, anéis concêntricos |

## 6. Animações

### 6.1 Hero — os seis clipes

Descritos no item 1. São vídeos gravados, não animação autoral: `.webm` (VP9) com poster `.webp`, autoplay mudo, loop, troca por aba, respeitando `prefers-reduced-motion` (mostra só o poster).

### 6.2 Pain — "a pilha montada à mão vira CompozyOS"

| Campo | Valor |
| --- | --- |
| Onde | Seção `pain`, `data-od-id="pain-visual"`, à direita do título "Anyone can prompt an agent. Keeping one working is still an engineering project." |
| Slot | 6 das 12 colunas, 440 px de altura mínima |
| Hoje | placeholder "Animation · DIY stack → CompozyOS"; o CSS/JS do colapso antigo (`.parts` → `.core`) ficou dormente e sai junto com a nova animação |

O que a animação precisa contar, nessa ordem:

1. **A pilha.** Nove peças que hoje o builder monta sozinho: agent CLI, loops, triggers, cron e webhooks, memory, permissions, approvals, observability, glue scripts. Cada uma é um objeto pequeno e distinto (ícone + rótulo curto), espalhadas, levemente tortas — dá para sentir que foram encaixadas à mão.
2. **O colapso.** As nove convergem para o centro e viram um único bloco CompozyOS (o símbolo + o nome). Não é explosão nem partícula: é encaixe, com peso. 900 ms de espera após entrar no viewport, ~1,2 s de movimento, easing expo-out.
3. **O repouso.** O bloco fica, com um brilho baixo atrás; a frase "replaces the pile" aparece pequena embaixo. Um botão Replay discreto no canto reexecuta.

Restrições: paleta do produto, um acento; sem loop infinito (roda uma vez, replay manual); sob `prefers-reduced-motion` mostra o quadro final estático com as nove peças listadas acima do bloco e uma seta entre eles; funciona sem JS como o quadro estático. Pode ser CSS/JS autoral (como era) ou um vídeo/Lottie — se for vídeo, entregar também o quadro final como imagem para o fallback.

### 6.3 O que não é asset

Reveal-on-scroll, as abas do hero, a barra de progresso das abas e o collapse do DIY-stack no comparativo são código em `landing.js`/`landing.css` e não entram nesta lista.

---

## Checklist de entrega

- [ ] 1 poster do hero + 6 clipes (`hero-poster.webp`, `demo-1..6.webm` + posters)
- [x] 5 spots (16:9 ×3, 21:9 ×2) — gerados 2026-09-10 com `gpt-image-2.5-sunburst` (skill `imagegen`, endpoint `edit`, duas referências de estilo: Dribbble 27555299 e 27087150); variantes escolhidas: implement v2, review v3, briefing v3, release v2, gate v1; depois um passe de edição (`--input-fidelity high`) trocou só o fundo para a cor do card, #1F1E1C, e as finais são bg-pass implement v1, review v1, briefing v3, release v3, gate v1
- [ ] 6 capturas 16:10 (uma reaproveita o poster do hero)
- [x] 3 painéis conceituais — ext-cartridges v3 (depois re-editada com o chão em #171615, bg-pass v1, e aplicada como backdrop da seção), bridges-inflow v2, closer-shell v1
- [x] 4 atmosferas — hero-wave v1, backdrop-traces v3, backdrop-orbit v1, backdrop-radar v1
- [ ] animação do pain (+ quadro final estático)
- [ ] apagar o capítulo "image placeholders" no fim de `landing.css` quando as capturas e o poster do hero entrarem (os 12 `.webp` gerados já substituíram os antigos de mesmo nome; `.ph__tag` já saiu)

Limite conhecido do set gerado: o CLI da skill só aceita os tamanhos legados para modelos diferentes de `gpt-image-2`, então os arquivos saíram em 1536×1024 / 1024×1536 / 1024×1024, abaixo do piso deste brief (≥ 2400 px hero, ≥ 1600 px spots). Para produção: upscale 2× dos escolhidos ou regerar os mesmos prompts em `gpt-image-2` nos tamanhos do brief. Prompts e as 36 variantes ficaram em `/tmp/compozy-landing-gen/` (não versionado).
