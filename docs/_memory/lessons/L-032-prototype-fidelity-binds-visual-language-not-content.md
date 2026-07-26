# L-032 — Prototype fidelity binds visual language, not content

**Class:** Frontend / Design system / Process
**Date discovered:** 2026-07-20
**Evidence sources:** os-shell task_03 (commit `401633d7e`); `.compozy/tasks/os-shell/memory/task_03.md` R4; implementation review of 2026-07-20; working-tree fix removing the `Logo` `menubar`/`glyph` variants.

## Context

os-shell task_03 implemented the OS menubar in Visual Contract Mode against `docs/design/opendesign/os/agh-os-v2.html`. The prototype's `.mb-logo` — a 16×16 accent tile with a center dot, placeholder chrome art — was treated as a normative parity obligation. The reuse gate ("check `packages/ui/src/index.ts` before authoring"; `compozy-ui-reuse/no-shadow-ui-primitive`) forbade a local mark, and loop memory crystallized R4: "always reuse `@agh/ui` `Logo` — never a local MenuBarMark". The only resolution satisfying every gate at once was growing the shared brand primitive: `Logo` gained `menubar` (and `glyph`) variants reproducing placeholder art, shipped with story + test + a passing VC bundle (VC-04, zero authorized differences).

No individual rule was wrong; the crossing was. Visual Contract fidelity defaulted to `normative` on every axis including marks and content, and nothing ranked the authorities when parity, reuse, and placement collided — so the agent optimized for the cheapest all-gates-green path.

## Root cause

A prototype is lossy by construction — it carries demo data, simplified copy, stand-in marks, and omits product content. Doctrine that binds every axis as normative makes agents faithfully reproduce the losses: placeholder art becomes brand inventory, demo data becomes UI truth, omissions become deletions. The missing rule was an authority scope: the reference owns visual language (layout, anatomy, typography, tokens, motion); runtime truth owns content/data; `COPY.md` owns labels/copy; the `@agh/ui` brand inventory owns marks.

## Rule

> A visual reference is normative for visual language only. Content, data, copy, brand marks, and which controls/views exist are owned by runtime truth, `COPY.md`, the brand inventory, and existing product surfaces. A prototype placeholder or omission is never an instruction — resolve toward the canonical owner and record an authorized difference. Reference artwork never extends brand primitives.

## Anti-pattern

- Growing a shared brand primitive (a `Logo` variant) to reproduce prototype placeholder art.
- Deleting or stubbing existing product content because the prototype does not show it.
- Rendering demo counts or fixture copy from a mock instead of runtime projections (SD-007).
- Loop memory crystallizing a gate-conflict resolution ("never a local X") without naming the authority that decided it.

## Source

- `.compozy/tasks/os-shell/memory/task_03.md` R4 (original crystallization; revised 2026-07-20) and `.compozy/tasks/os-shell/_techspec.md` visual-contract delta #12.
- Encoded in: `.agents/skills/agh/agh-ui-screenshot/references/visual-contract.md` (visual-language scope), `.agents/skills/agh/agh-design/SKILL.md` (§Named visual contracts + Error Handling), `.agents/skills/cy-final-verify/SKILL.md` §Visual Contract Parity, root `CLAUDE.md` §Design System, `packages/ui/CLAUDE.md` tripwires, SD-007.
