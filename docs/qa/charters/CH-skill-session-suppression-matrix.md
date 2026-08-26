# CH-skill-session-suppression-matrix: Keep two sessions open and prove neither one lost a skill it should have had

```yaml
charter:
  id: CH-skill-session-suppression-matrix
  mission: "As Théo, keep sessions open side by side under different providers and profiles while an operator changes sources underneath them, and prove that a skill is omitted from a prompt only when that session's own provider already reads it — that every omission is explainable, every listed skill is still invocable, and no session inherits another one's catalog."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-use-absorbed-skills-in-a-session
  scenarios: [ET-skill-session-source-injection, ET-session-command-catalog-parity, ET-session-composer-skill-chip]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Open sessions under Claude, OpenClaw, Hermes, their canonical aliases, and one unknown provider at the same time, with the same skills visible to all of them. Each session must suppress only the winning roots its own provider reads natively, at per-folder granularity — a provider that reads only the workspace-level universal folder must not suppress the global one — and the unknown provider must suppress nothing at all."
      - "For every omission, get the reason out of harness diagnostics before accepting it. A skill missing from a prompt with no diagnostic naming the skill, source, and provider is the finding, whether or not the omission was correct."
      - "Confirm suppression is prompt-only: the same skill must stay listed in the picker, readable through the skills APIs, enable/disable-able, and present in the shadow records. Then invoke a suppressed skill explicitly with / and confirm its content arrives in full — the explicit request wins."
      - "Add a homonym across two roots and work both tabs: both must be listed and distinguishable in the picker with their origin labels, each qualified form must reach its own physical skill, and the composer chip for one must not be mistakable for the other. Compare the web menu against compozy session commands -o json and the HTTP/UDS route on the same revision, then re-walk the wrong-workspace fence."
      - "With one session still open, switch the remembered profile and confirm that session keeps the catalog of the profile it was created with. Then relocate or isolate a provider home and confirm the operator-home folders stop being that session's native roots, so their skills are injected normally. Finish by disabling a source between picking a skill and submitting: the drift rejection must be deterministic, inject nothing partial, and leave the transcript unmutated."
    must_avoid:
      - "Judging suppression by reading the prompt alone. The claim is per-winning-root, not per-name — a skill whose winning copy is a Compozy root must be injected even when a same-named copy exists in a native folder."
      - "Source policy editing, per-root diagnostics, and expose actions — they belong to their own charters; a finding there is a follow-up, not this debrief."
      - "Reloading a tab to make a stale row disappear. If the picker only tells the truth after a manual reload, that is the finding."
```

## Selection rationale

Third session of the targeted tier and the one whose failure is quietest: a skill that silently is
not in the prompt looks exactly like a skill that was never installed. Safety Invariant 6 draws the
line this session polices — suppression filters prompt sections A and B only, while the command
catalog projection, the settings and read APIs, enable/disable, shadows, and explicit `/` expansion
are never filtered, and an unknown session provider means no suppression at all. ADR-009 makes
suppression an internal provider-to-native-roots mapping rather than user configuration, ADR-010
makes it automatic with no config key to turn it off, and together they mean the operator has no
switch to flip when it goes wrong — diagnostics are the only recourse, which is why "every omission
is explainable" is a must-try rather than a nicety.

Safety Invariant 7 and ADR-016 carry the other half: the pre-overlay candidate projection identifies
roots by an opaque stable `RootID` derived from resource scope, stable profile id, stable workspace
id, root kind, and canonical directory — never from a profile name or list position — and it stays
deterministic for a configuration generation, with a generation change invalidating in-flight
invocations through source-drift rejection. Safety Invariant 12's cache keying is what stops one
session from serving another profile's catalog.

The Multi-Tab Tour is the deliberate choice over Interrupt here. Every one of those invariants is
about state shared across concurrent sessions — two providers, two profiles, one operator changing
policy underneath both — and the bugs live in the seams between them, not in a single interrupted
call. `ET-session-command-catalog-parity` and `ET-session-composer-skill-chip` ride along because
this diff reset them: the catalog projection and the composer's trailing slot are the same code paths
the suppression work moved through, and leaving them to a later cycle would mean walking the same
surface twice.

Théo owns it rather than Dora because the person who loses here is the one mid-conversation who
reaches for a skill and cannot find it — not the administrator who configured it.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
