# Persona Fidelity

The guardrails that keep a session *real*. The moment the runner starts behaving like an evaluator with privileged access, the session stops measuring the product and starts measuring the runner's ability to make dashboards green. These rules are sticky: when one blocks you, the fix is to change the session, never to relax the rule.

## Contents

- The public-interface rule
- Forbidden framings
- Stall is a finding
- The allowlist
- Fidelity vs the fix loop
- Rule governance
- Anti-patterns

## The public-interface rule

Everything the session does goes through surfaces a real user can reach:

- **Interact** only via the product's UI, CLI, or documented public API — as the persona, with the persona's knowledge.
- **Verify** only via read paths the product exposes to users (fresh loads, list views, emails it sends, exports it offers).
- **Never**: read source code to learn what *should* happen mid-session; query the DB to confirm a write; call internal endpoints; flip feature flags; edit fixtures to make a path reachable. If the plan needs product knowledge, that's planning work (`qa-report`) done *before* the session, not during it.

Knowledge asymmetry is the point: the persona knows what the product taught them, nothing more. A New User who "happens to know" the admin URL is not a New User.

## Forbidden framings

Wherever the session produces text a product surface or agent under test will consume (form inputs, chat prompts to an AI product, support messages, seeded content), evaluator framing is forbidden — it changes the product's behavior and invalidates the result:

- Naming the activity: "as a QA tester", "this is a test", "I am verifying/auditing", "test case", "expected result".
- Grading language: "pass/fail", "go/no-go", "acceptance criteria", artifact ids (`BUG-`, `CH-`, `J-` prefixes) leaking into product-facing text.
- Instructing the product to self-report: "confirm that you...", "list what you did so I can check".

The persona writes what a real person with their goal would write. If a session note needs meta-language, it goes in the session log — never into the product.

This matters double for **agentic products** (chatbots, copilots, multi-agent systems): an agent told it is being tested behaves differently. The session's prompts must be indistinguishable from real product usage — one in-persona kickoff with a real goal, then observation.

## Stall is a finding

When the product hangs, a button does nothing, an agent under test goes silent, or a flow dead-ends:

1. Capture the state (screenshot, elapsed time, what the persona tried).
2. Record the verdict (`fail` or blocked) and file/update the bug.
3. Move on to the next step the persona could realistically reach, or end the session leg.

**Never** nudge the product past the stall — re-prompting an agent, force-refreshing until it works, or retrying until a race un-sticks masks exactly the defect the session exists to catch. One clean retry from a fresh session is legitimate (real users retry once); record that the first attempt failed either way.

## The allowlist

Real product work uses words that look meta but aren't — don't over-block:

- "review", "approve", "verdict" — when the *product* is about reviews/approvals, using those flows is real usage.
- "test" — when it names a real artifact ("run `go test`", "the test environment toggle") inside a persona whose job includes it.
- Bug reports *filed through the product's own feedback feature* — a real user filing feedback is realistic usage; write it as the persona would.

The line: meta-language *about this QA session* never touches the product; the product's own vocabulary is fair game.

## Fidelity vs the fix loop

Fidelity governs the **session**; the fix loop (routed at Step 7 of the SKILL) happens **between** sessions:

- Inside a session, the persona never fixes anything — a real user can't patch the product.
- After a session ends, the governor may authorize a bounded fix; then the impacted journey is **re-walked from scratch, in persona** — a fresh session, not a resumed one. The persona doesn't "know" the bug was just fixed; the Recovering User persona exists precisely for that re-walk.

## Rule governance

- Rules are sticky, sessions are variable: when a rule blocks a session, rewrite the charter/persona/entry — never weaken the rule in the moment.
- Relaxing or adding a rule happens in review, in this file, with the incident that motivated it written down.
- A fidelity violation discovered mid-run invalidates the affected verdicts: reset them to `Pending`, note the violation in the report, re-run clean.

## Anti-patterns

- **The omniscient persona** — using URLs, flags, or vocabulary the persona was never taught.
- **Verification leakage** — DB peeks and code reads sneaking in as "just double-checking". The independent read path must be user-reachable.
- **Prompt-nudging agentic products** — "are you still there?", "please continue with the task" — the stall was the data.
- **Grading language in product inputs** — the fastest way to make an AI product behave unnaturally.
- **Relaxing rules under deadline** — a green matrix produced by loosened fidelity measures nothing; the next release inherits the bugs.
