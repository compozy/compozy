# CH-skill-sources-diagnostics-truth: Point Compozy at hostile folders and make it explain itself

```yaml
charter:
  id: CH-skill-sources-diagnostics-truth
  mission: "As Dora, feed the scanner the worst folders a real machine produces — unreadable, enormous, full of dead links, links reaching into another workspace, and skills written for other tools — and prove that every root explains its own state, that no count is ever presented as measured when it was not, and that a missing skill is always diagnosable from the product."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-diagnose-skill-sources
  scenarios: [ET-skill-source-diagnostics-cli, ET-skill-source-symlink-containment, ET-skill-ecosystem-frontmatter-quiet]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Build the hostile fixture before reading anything: one absent folder, one folder with permissions removed, one root far over the per-root cap, one root full of dangling, escaping, and cyclic first-level links, one name collision across two roots, and one root of skills authored for another tool. Then read compozy skill sources and its JSON and confirm they carry the same facts — the header vocabulary, the always on / enabled / disabled states, the absent / unreadable / truncated / links skipped / collisions notes, and the scope and overrides footer."
      - "Check the unreadable root specifically: it must show no count at all. A zero here is worse than an error, because a zero looks measured. The absent root must read as absent rather than zero, and the truncated root must keep its real scanned count beside the cap. Every other root must keep loading normally throughout."
      - "Walk each skipped-link class separately and confirm it is contained, not fatal: dangling target, target outside every trusted root, first-level cycle. Then the isolation cases — a link from one workspace's folder into another workspace's configured folder, and a link from one profile's projection into another profile's roots — must both be refused with a diagnostic, and two profiles with identical source configuration must still get their own catalog."
      - "Reproduce the installer layout (canonical body plus per-tool symlink) and confirm the skill appears exactly once by resolved path, attributed to the highest-precedence root that reached it, and still loads when only the linking preset is enabled. Expose a skill into a scanned root and rescan: CompozyOS's own link must never produce a duplicate or shadow its own canonical skill."
      - "Enable the ecosystem root and count the warnings. Recognized fields from other tools must produce none, and a tool-allowance field must be provably inert — invoking the skill must not widen or narrow what the session can do. Then confirm the signal survives: an unknown field still names itself, a SKILL.md missing its name is rejected per-skill while its neighbours load, and a directory with no SKILL.md is ignored rather than treated as an error. Finally repair each fault and confirm the stale diagnostic clears with no restart."
    must_avoid:
      - "Re-filing BUG-20260729-skill-workspace-error-mapping. A missing workspace returning HTTP 500 instead of a stable not-found is already open against the same resolver these --workspace paths use; if it reproduces, append to that file rather than minting a new id."
      - "Reading only the JSON. Half this session's value is that the human table and the structured output agree; a divergence between them is a finding that only appears if both are read."
      - "Source policy writing, live-apply convergence, and expose lifecycle — each has its own charter."
```

## Selection rationale

The Garbage Tour is the literal match here: this journey exists for the folders a real machine
actually has, and the scanner's contract is entirely about surviving them without lying. Safety
Invariant 1 requires per-projection containment — a followed symlink loads only when its resolved
target lies inside that exact projection's trusted roots, never another profile's or workspace's —
with escaping, dangling, and cyclic links skipped per entry and never fatal to the root. Safety
Invariant 2 requires one physical directory to yield exactly one catalog entry per scope, attributed
to the highest-precedence root that reached it, and forbids CompozyOS's own expose links from
shadowing their canonical skill. Safety Invariant 10 runs content verification on every non-bundled
skill on every load, blocking that skill only and never the root — which is why the per-root
verification summary must be legible enough to explain "scanned five, shows three". Safety Invariant
11 holds the per-root caps and requires truncation to be marked in the same pass and surfaced in
every sources read model.

ADR-012 is the decision under all of it: follow first-level symlinks and deduplicate by realpath,
which is what makes the popular installer layout work and what makes containment load-bearing rather
than theoretical. US-014's promise — that "skill missing" is always diagnosable from product
surfaces — is the user-facing claim being tested, and US-015's quiet-loading promise is here because
warning noise is what destroys diagnosability in practice: a hundred warnings about fields Compozy
deliberately ignores makes the one real warning unfindable.

Dora owns it because she is the one who gets asked why a skill is missing and has to answer from the
product rather than from a log tail. `ET-skill-source-symlink-containment` and
`ET-skill-ecosystem-frontmatter-quiet` are new in this cycle — the containment and warning-noise
promises were shipped by tasks 02 and 03 without a scenario of their own — and they share the fixture
with `ET-skill-source-diagnostics-cli`, so walking them apart would mean building the same hostile
tree three times.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
