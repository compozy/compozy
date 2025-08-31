# Deep Plan Output Template

Use this template for the final planning document and the initial `_phase2` draft. Replace bracketed prompts with project-specific content.

```markdown
🗺️ Deep Plan Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎯 Objectives

- [Goals and success criteria]

📦 Scope

- In: [what’s included]
- Out: [what’s excluded]

🧭 Assumptions

- [Explicit assumptions]

⛓️ Constraints

- [Performance, security, compliance, backwards compatibility]

⚠️ Risks & Mitigations

- [Risk → mitigation]

🔗 Dependencies

- [Internal/external deps, owners, readiness]

🧩 Architecture & Context Map

- [Key modules, interfaces, boundaries]

📐 Standards Compliance

- Rules satisfied: [@api-standards.mdc, @architecture.mdc, ...]
- Deviations (if any): [explain + compliant alternative]

📅 Milestones

1. [Milestone name] — [exit criteria]
2. [...]

🧱 Work Breakdown Structure (WBS)

1. [Task] — [deliverable] — [estimate]
2. [...]

✅ Acceptance Criteria

- [Measurable verifications/tests]

🚀 Rollout & Ops

- [Release plan, feature flags, monitoring, rollback]

📝 Decision Log

- [Decision] — [Rationale] — [Date]

🤝 Multi‑Model Synthesis (gemini-2.5-pro + o3)

- Agreements: [shared conclusions]
- Divergences: [differences]
- Final Rationale: [why selected approach]

❓ Open Questions

- [Unknowns to resolve]

Returning control to the main agent. No changes performed.
```

## Save Block Format

After printing the markdown plan, emit the structured save block with the same content and a timestamped filename:

```xml
<save>
  <destination>
    ./ai-docs/deep-plans/{UTC_YYYYMMDD-HHMMSS}-{safe_name}.md
  </destination>
  <format>markdown</format>
  <content>
  [PASTE THE FULL PLAN MARKDOWN HERE]
  </content>
  <audience>main-agent</audience>
</save>
```

For the initial Phase 2 draft synthesis, use the same structure but append `_phase2` to the filename before the extension.
