# CH-skill-sources-settings-web: Abuse the sources form and check every number on it was really measured

```yaml
charter:
  id: CH-skill-sources-settings-web
  mission: "As Dora, mistreat the Settings > Skills sources section — paste garbage paths, add duplicates, save while the runtime is down, switch scopes mid-draft — and prove every row's state and count came from what the daemon measured, that a rejected save keeps the draft and quotes the daemon, and that a person who has never heard the word preset can still read the page."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-absorb-skills-from-other-tools
  scenarios: [ET-web-skill-sources-settings]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Point the page at a fixture home with one populated universal folder, an absent .claude/skills, an unreadable custom root, and an over-cap root. Every total and every folder line must trace to the daemon's sources[].roots[] — the unreadable folder shows no count at all, the absent one reads as no folder yet, the over-cap one keeps its real count beside the partial-scan sentence. Cross-check each number against compozy skill sources -o json; a page-computed number that happens to be right is still the finding."
      - "Abuse the custom-directory editor: paste a ten-thousand-character path, a path with a trailing newline, emoji and RTL text, a project-relative path at user scope, and the same path twice. Each must produce its inline error — duplicate_skill_source, invalid_source_path — with the draft preserved and nothing applied, and the field must not freeze or silently truncate."
      - "Toggle a preset and confirm the count and the composer picker follow within two poll intervals without a reload. Confirm the compozy row is always on with no switch, and that disabling every optional source with no custom entries produces the explicit defaults-only state rather than an empty table."
      - "Break the save deliberately: stop the runtime and confirm counts and existence degrade explicitly while the policy stays editable; force a rejected save and confirm the daemon's message is quoted verbatim, the draft survives, nothing was applied, and retry works. Then switch to workspace scope — both keys start inherited, editing one makes only that key custom, Use inherited returns it while the other stays untouched — and verify the other workspace through the API. Under a named profile the selected workspace keeps that exact profile with no write affordance; at agent scope the section reads without one."
      - "Read the page as someone who has never heard the words preset or root. Every row, state, and error must say what it means and what to do; a label that only makes sense to whoever implemented it is a finding, not a preference."
    must_avoid:
      - "Re-filing BUG-20260729-skill-agent-default-selection or BUG-20260729-skill-policy-normalized-dirty. Both are open against this same page — agent scope opening with an unavailable identity, and a saved policy staying marked unsaved through the same inline-save controls. If the sources section inherits either symptom, append to those files."
      - "Asserting on copy strings where a stable selector exists. The settings-page-skills-sources family covers the rows, counts, disclosures, scope switches, inheritance controls, save state, and errors."
      - "Mobile and narrow viewports — this is a desktop-only surface by the project's own persona policy, and this diff added no layout."
```

## Selection rationale

The Garbage Tour, because this is the cycle's only free-text input surface and the truthful-UI claim
is strongest exactly where the input is worst. The `_uiux.md` contract for S1 is unusually strict:
rows and counts render only daemon-reported state from the `sources[]` envelope, a directory that is
absent renders as an explicit absent state and never as a zero that looks measured, an unreadable
root omits counts entirely, runtime-unavailable suppresses counts while keeping the policy editable,
and ineligible affordances are absent rather than disabled. Every one of those is a claim about what
the page refuses to invent, and the only way to test a refusal is to give it something to invent
from.

ADR-006 is the decision this settles on the web — the workspace override ships with full CLI, API,
*and* web management — and the per-key inheritance walk is what proves the tri-state on the wire
(absent means untouched, null clears, an array sets) survives the round trip into a form. The PR #457
amendment adds the exact-profile lens: drafts and queries key by exact profile, and the repository
profile layer is read-only, so a named profile must show no write affordance rather than a disabled
one.

Dora owns the session and also carries the plain-language lens here, which is a deliberate compromise
recorded rather than hidden: this cycle has no separate newcomer charter, so the experiential
dimension rides as a must-try on the surface where new vocabulary actually appears. The two open bugs
on this page are named in `must_avoid` because both are plausible re-finds through the shared
inline-save controls and the shared scope selector, and a split history across two ids would cost
more than the session gains.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
