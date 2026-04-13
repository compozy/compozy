# Council of Advisors Reference

Run the council as a real embedded subagent workflow, not as inline roleplay. The facilitator remains inside `cy-idea-factory`, but every advisor is dispatched through the host-owned `run_agent` tool using canonical reusable-agent ids.

## Runtime Contract

- The standard council roster is provisioned by `compozy setup` under `~/.compozy/agents/<name>/`.
- A workspace may override any advisor by defining `.compozy/agents/<name>/`.
- Dispatch advisors by id, never by driver-specific paths:
  - `pragmatic-engineer`
  - `architect-advisor`
  - `security-advocate`
  - `product-mind`
  - `devils-advocate`
  - `the-thinker`
- If `run_agent` is unavailable or any selected advisor cannot be resolved, stop and tell the user to run `compozy setup`.

## When to Use

- Making high-impact product, architecture, or scope decisions with real trade-offs
- Stress-testing a V1 proposal before writing the final idea draft
- Comparing multiple viable options where stakeholder priorities differ
- Preserving dissent instead of collapsing tension into false consensus

## Embedded Mode

`cy-idea-factory` always runs council in embedded mode:

- skip user-facing context confirmation because the parent workflow already established the dilemma
- skip final decision capture because `cy-idea-factory` owns the downstream draft and ADR
- return synthesis material that the parent workflow can extract into scope, risks, V1 exclusions, and V2 opportunities

## Advisor Selection

Select 3-5 advisors based on dilemma complexity:

- **3 advisors** for binary choices or a narrow trade-off axis
- **4 advisors** for multi-factor decisions with 2-3 competing concerns
- **5 advisors** for broad, multi-faceted dilemmas

Selection rules:

- always include `devils-advocate` when consensus forms quickly or the user explicitly wants stress-testing
- include `the-thinker` when the framing itself may be the constraint
- prefer the smallest roster that still produces real tension

## Phase 1: Opening Statements

Dispatch all selected advisors through `run_agent`. Run them in parallel when the runtime supports parallel tool calls.

Each advisor receives:

1. The refined dilemma and explicit constraints from the idea workflow
2. The roster of other advisors in the session
3. The instruction: "Deliver your opening statement (2-3 paragraphs) ending with a one-line **Key Point**."

Render results as:

```markdown
## Opening Statements

### [Advisor Name] — [Archetype]

[Opening statement]

**Key Point:** [One-line summary]
```

## Phase 2: Tensions and Rebuttals

Read the openings and identify 2-4 genuine tensions. A real tension has Side A, Side B, and meaningful stakes.

For each tension:

1. Re-dispatch the opposing advisors through `run_agent`
2. Require steel-manning first
3. Then require a rebuttal plus one of: concede, partially concede, hold firm

Use this prompt shape:

```text
Steel-man [opponent]'s position in 1-2 sentences, then deliver your rebuttal in 1 paragraph.
State whether you concede, partially concede, or hold firm, and why.
```

Record the debate as:

```markdown
## Core Tensions

| Tension | Side A (Advisor) | Side B (Advisor) | Facilitator Note |
| ------- | ---------------- | ---------------- | ---------------- |
| ...     | ...              | ...              | ...              |

### Key Concessions

- **[Advisor A]** concedes to **[Advisor B]** on [point] because [reason]
- **[Advisor C]** holds firm on [point] because [reason]
```

## Phase 3: Position Evolution

Track how positions moved after rebuttals:

```markdown
## Position Evolution

| Advisor | Initial Position | Final Position | Changed? |
| ------- | ---------------- | -------------- | -------- |
| ...     | ...              | ...            | Yes/No   |

**Key Shifts:**

- [Who changed and why]
```

## Phase 4: Synthesis

Produce the final synthesis in this order:

```markdown
## Council Synthesis

### Points of Consensus

- ...

### Unresolved Tensions

| Tension | Position A | Position B | Trade-off |
| ------- | ---------- | ---------- | --------- |
| ...     | ...        | ...        | ...       |

### Recommended Path Forward

**Primary Recommendation:** ...

**Rationale:** ...

**Dissenting View:** ...

### Risk Mitigation

- ...
```

## Extraction Guide for `cy-idea-factory`

After synthesis, the parent workflow should extract:

- the recommended V1 scope
- explicit out-of-scope items for V1
- the strongest risk factors and hidden dependencies
- priority guidance relative to other work
- one stretch direction worth considering for V2+

## Debate Protocols

- **Steel-man first**: every rebuttal starts with the strongest version of the opposing case
- **Evidence required**: no bare assertions
- **No false consensus**: preserve live disagreement in the synthesis
- **Authentic voices**: each advisor argues from its real priorities
- **Concession discipline**: if someone moves, record what changed their mind

## Failure Handling

- If an advisor returns out-of-character content, re-dispatch once with a protocol reminder.
- If the failure repeats, record it in the synthesis and proceed with the remaining advisors.
- If fewer than 2 real tensions emerge, note that the dilemma may be lower-stakes than expected and continue with a condensed synthesis.
