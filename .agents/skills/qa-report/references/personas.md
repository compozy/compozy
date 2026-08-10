# Personas

A persona is the answer to *"who am I being right now?"* QA without a persona drifts into developer mindset — testing what the system can do instead of what a user would actually try. This reference is the **methodology and seed catalog**; a project's actual personas are instance data living in `<qa-docs-path>/personas.md`, derived once and evolved as the audience changes.

## Contents

- Why personas
- Deriving project personas (instance data)
- Seed catalog (New / Power / Casual / Mobile / Accessibility-Reliant / Recovering User)
- Persona attributes (YAML schema)
- Picking the right persona for a charter
- Anti-patterns

## Why personas

- A session run as "anyone" optimizes for the tester's reflexes, not the user's needs.
- Different personas surface different defects: a new user finds onboarding friction; a power user finds shortcut regressions; a mobile user finds touch-target failures.
- The persona is the leash: when you catch yourself working around a problem instead of recording it, the persona pulls you back into role.

## Deriving project personas

Write 3-6 personas to `<qa-docs-path>/personas.md`, each grounded in the product's **real audience** — not copied verbatim from the seed catalog:

1. Start from the product's value proposition: who pays, who uses daily, who arrives for the first time, who comes back after something went wrong.
2. Map each to the closest seed persona and adapt: give it a name, the product-specific goal it pursues, its device/network reality, and its patience threshold.
3. Keep the seed persona's `name` in a `base:` field so charters and bug reports can be read across projects.
4. Include at least one Mobile-based persona when a mobile surface exists, and one Accessibility-Reliant persona unless explicitly out of scope (record the skip reasoning in the file).

Personas are durable: update them when the audience changes, not per cycle.

## Seed catalog

### New User (first-time visitor)
- **Familiarity:** zero. Has not seen the product before.
- **Motivation:** evaluating; will leave if confused within 60 seconds.
- **Device:** whatever they happened to be on — often mobile.
- **Patience:** very low. A spinner over 3 seconds feels broken; an unclear error sends them to a competitor.
- **What they reveal:** onboarding gaps, missing empty-state guidance, confusing copy, unclear primary action, broken first impressions.

### Power User (returning expert)
- **Familiarity:** daily use. Knows shortcuts, edge features, how to "abuse" the UI.
- **Motivation:** ship work fast. Tolerates ugly UI if it's efficient.
- **Device:** desktop, keyboard-driven, many tabs open.
- **Patience:** zero for speed regressions; high for visual rough edges.
- **What they reveal:** shortcut breakage, bulk-operation regressions, performance degradation, cross-tab state loss, undo/redo bugs.

### Casual User (returning, infrequent)
- **Familiarity:** a few visits. Remembers the goal, not the steps.
- **Motivation:** complete one task and leave.
- **Device:** mixed; often switches phone ↔ laptop mid-task.
- **Patience:** moderate, while the goal is in sight.
- **What they reveal:** discoverability ("where did that button go?"), cross-device continuity, save-and-resume bugs.

### Mobile User (touch-first)
- **Familiarity:** any. Defined by device.
- **Motivation:** quick action — often in transit, often one-handed.
- **Device:** small viewport, touch, possibly slow network.
- **Patience:** low. Closes the tab on a layout shift.
- **What they reveal:** touch-target size, 375px layout breaks, sticky elements covering content, unintended zoom, network-failure handling.

### Accessibility-Reliant User (assistive tech)
- **Familiarity:** any. Defined by interaction modality.
- **Motivation:** use the product on equal terms.
- **Device:** screen reader (VoiceOver / NVDA / TalkBack), keyboard-only, magnifier, voice control, or high-contrast mode.
- **Patience:** task-bounded. Abandons when announcements are incomprehensible.
- **What they reveal:** missing labels, focus traps, broken tab order, color-only signaling, modals that don't escape, unannounced dynamic content.

### Recovering User (returning after a problem)
- **Familiarity:** any. Defined by emotional context — the last visit ended badly.
- **Motivation:** check if it's fixed; trust is fragile.
- **Device:** often the same device that saw the failure.
- **Patience:** very low. Any sign of the previous failure triggers abandonment.
- **What they reveal:** stale error states, cached failure screens, half-applied fixes, "we're sorry" pages that loop.

## Persona attributes (YAML schema)

Record the persona row alongside every charter and bug:

```yaml
persona:
  name: <project persona name>
  base: <New User | Power User | Casual User | Mobile User | Accessibility-Reliant | Recovering User>
  goal: <the product-specific outcome this persona pursues>
  device: <desktop | laptop | tablet | phone-small | phone-large>
  network: <wifi-fast | wifi-slow | 4g | 3g | flaky>
  modality: <mouse-keyboard | touch | screen-reader | keyboard-only | voice>
  locale: <en-US | pt-BR | de-DE | ...>
  patience_seconds: <how long before abandoning>
```

The bug template's `Persona Affected:` field uses `name`.

## Picking the right persona for a charter

| Surface | Mandatory persona (base) | Recommended additional |
|---|---|---|
| Onboarding / first-run | New User | Mobile User |
| Settings / account | Casual User | Power User |
| Bulk operations / dashboards | Power User | Casual User |
| Mobile-only feature | Mobile User | Accessibility-Reliant |
| Recovery / error pages | Recovering User | New User |
| Public marketing / landing | New User | Accessibility-Reliant |
| Form-heavy flow | Casual User | Mobile User (autofill) |
| Multi-step wizard | New User | Power User (skip patterns) |

When in doubt: *"who is most likely to be hurt by this surface failing?"* — that's the mandatory persona.

## Anti-patterns

- **"Just a user"** — generic personas produce generic sessions. Pick one.
- **Persona-of-convenience** — choosing the persona that matches what you already wanted to test. Invert it: pick the persona, let it pick the test.
- **Single-persona cycles** — a full cycle covers at least 3 personas across its sessions, or large user segments go unverified.
- **Mixing personas mid-charter** — run one persona to completion; record any switch in the debrief.
- **Copying the seed catalog verbatim as project personas** — the seed is a scaffold; personas that don't name the product's real goals produce sessions that don't walk real journeys.
