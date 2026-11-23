# CH-skill-install-recovery: Recover an interrupted skill install and resolve a conflict from either home

```yaml
charter:
  id: CH-skill-install-recovery
  mission: "As Dora, interrupt a skill approval mid-install, restart, and see it rolled back; interrupt another and edit the skill by hand so it becomes a conflict, then resolve conflicts with --keep previous and --keep current, one from a second CompozyOS home on the same project."
  mode: charter-with-tour
  persona: {name: Dora, device: desktop, network: wifi-fast, locale: en-US}
  journey: J-31
  scenarios: [MS-web-stays-on-reports]
  tour: Bad Neighborhood Tour
  time_box_minutes: 60
  entry_path: "approve while the lab daemon is frozen (SIGSTOP) at a chosen journal phase → SIGKILL → restart → compozy agent skill review show; external edit while frozen after backup_staged → restart → approve refused skill_install_conflict → compozy agent skill install-conflict list|resolve from home A and home B"
  expected_evidence:
    - "After a kill before the commit point and a restart, the skill directory equals its pre-approval tree, no .compozy-* leftovers remain, and the review is still pending and approvable."
    - "A kill after the commit point finishes on restart: review approved, skill installed."
    - "With an external edit retained as a conflict: approve fails with skill_install_conflict naming the op; the Web viewer shows the resolve-command alert; screenshot."
    - "install-conflict list shows the op in both homes; resolve --keep previous restores the original and keeps the edited copy at .skill-conflict-<op>; resolve --keep current keeps the edit; the list is empty afterwards and approvals of that skill work again."
  guidance:
    must_try:
      - "Resolve one conflict from home B while home A's daemon is the one that created it."
    must_avoid:
      - "Do not edit journals, install logs or SQLite by hand; only freeze/kill the lab daemon and edit the skill files a user could edit."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
