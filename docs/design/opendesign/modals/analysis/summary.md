# AGH modal redesign research summary

> **Living authority:** `MODAL-STANDARD.md`, the 16 surfaces + `verify.mjs` in this folder, and `../design-system/patterns.html` § Modals. This summary is research history; open questions below are not a live backlog.

## Research Question

Nós temos alguns modais aqui que foram criados pra melhorar os modais de criação e edição das entidades. Porém tem ainda vários modais que estão antigos e bem ruins, não tão seguindo esse padrão novo que a gente fez pra tasking, trigger e job. Pra task, trigger e job, nós criamos modais mais bonitos, o cabeçalho e tudo mais, o conteúdo, o formulário.

E agora a gente precisa alinhar esse nível, esse padrão de modal pra criação e edição de itens pra outras páginas. Pra todas as páginas principais, os módulos principais ali que tem algum tipo de ação de adicionar ou editar, a gente precisa melhorar esses modais. Você tem que usar sub-agentes pra primeiro explorar o projeto e identificar quais são esses modais, quais são as informações e integrações de dados do backend que eles têm pra não acabar implementando algo que não existe. Depois criar os designs desses modais.

Se você quiser criar uma pasta aqui e colocar todos eles, cada HTML dentro, é melhor ainda. Daí fica mais fácil a gente ver esses designs depois. Pode fazer isso. Temos que ter um padrão impecável e conseguir dar essa melhoria nos modais e também diminuir o level de complexidade.

Se você ler, por exemplo, no modal que criamos de task, ele tem uma aba ali simples e avançada porque o modal de task antes era muito complexo, era muito informação. Então nós criamos essa visualização pra melhorar, pra ficar mais intuitivo.

Essa ideia de usuário-operador nem sempre é bom então a gente está começando a ter mais essa ideia de usuário final também conseguir cadastrar as coisas aqui. Se você vê que um modal está muito complexo, muito denso, crie uma versão simplificada, melhore o I/O/X dele dessa forma e vamos tentar seguir ao máximo os padrões.

References: `create-task-redesign.html`, `create-job-redesign.html`, `create-trigger-redesign.html`.

## Slice Map

| Slice | Question | Finding |
| --- | --- | --- |
| `01 – web-modal-inventory` | Which production create/edit dialogs and sheets exist, and how are they wired? | Task, Job, and Trigger are the only complete Tier-1 references. Agent and Bridge create are legacy wizards; Knowledge, Session, Bridge edit, Network, and shared Settings editors are the main alignment targets. Provider and Loop should remain sheets. |
| `02 – runtime-contracts` | Which fields, validations, lookups, secrets, and transport contracts are real? | The daemon exposes a finite contract-backed set. Required cores support Simple mode; optional policy, retry, routing, secret, and metadata fields belong in Advanced mode. Secret values are write-only and edit identities are often immutable. |
| `03 – modal-reference-system` | What reusable modal system is encoded by the three references and shared UI primitives? | One shell and two body archetypes emerge: single-column Simple/Advanced and form plus truthful live preview. Runtime primitives override visual drift in the static references. |

## Convergences

- All three analyses support a shared entity-editor shell: ruled header and footer, icon well, domain eyebrow, clear description, one primary action, and tokenized geometry. See `01_analysis_web-modal-inventory.md` and `03_analysis_modal-reference-system.md`.
- Simple/Advanced is justified by both UX evidence and the request contracts: a small required core is followed by an optional operator-focused tail. Agent and Bridge benefit most. See `01_analysis_web-modal-inventory.md` and `02_analysis_runtime-contracts.md`.
- Provider/model catalogs, bridge providers, workspaces, channels, tools, skills, and vault refs already have lookup endpoints. Designs should use selectors and status rows, not free-text substitutes. See `02_analysis_runtime-contracts.md`.
- Write-only secret handling is a cross-domain contract. Create uses set-value controls; edit shows presence and rotation without ever rendering the stored value. See `02_analysis_runtime-contracts.md`.
- The runtime token system and exported primitives win over static-reference drift: selected cards stay neutral, scrim/radius/size use tokens, focus-visible states remain intact, and accent side rails are removed. See `03_analysis_modal-reference-system.md`.

## Divergences

- The static references use accent-filled selected cards and accent side rails, while `RadioCard`, `DESIGN.md`, and the migration checklist reserve accent for primary action. The modal library will follow the production primitive contract.
- Agent and Bridge create currently use wizards. The Web analysis treats a re-skinned wizard as viable, but the contract analysis shows a stable small core. The modal library will use Simple/Advanced and keep step-scoped validation as inline field validation.
- Provider and Loop configure are side sheets because the list/detail context matters. They should not be forced into centered dialogs for superficial consistency. Loop configure is already reference-grade and needs no new mock.
- Session creation is a launcher, not CRUD. Its design will use the same shell but will not imply a later edit modal.

## Risks & Open Questions

- Do not add controls not accepted by the real request contracts.
- Do not prefill or reveal write-only secrets; show presence, set, replace, and recovery states only.
- Preserve loading, stale, empty, and error states for provider/model catalogs in Agent and Session flows.
- Lock immutable edit fields such as Bridge platform/provider/scope and Network channel name/membership.
- Surface version/digest conflicts for stale edits where contracts use optimistic concurrency.
- Keep full Loop authoring outside a modal; only its existing configuration sheet fits this pattern.
- Keep marketplace trust/install confirmations and destructive confirmations outside the entity-editor redesign wave.

## Outcome (vs. recommended steps)

1. ~~Define one static artifact shell…~~ → Done: `modal-system.css` / `MODAL-STANDARD.md`.
2. ~~Design highest-value legacy targets…~~ → Done: 16 surfaces linked from `index.html`.
3. Bind fields to contracts → Design-side binding in HTML + `02_analysis_runtime-contracts.md`; production TechSpec still required for `web/` migration.
4. ~~Preserve Provider as sheet; Loop configure compliant~~ → Reflected in library scope note on `index.html`.
5. ~~Launcher index~~ → `index.html`.

## Index

- `modals/analysis/01_analysis_web-modal-inventory.md`
- `modals/analysis/02_analysis_runtime-contracts.md`
- `modals/analysis/03_analysis_modal-reference-system.md`
