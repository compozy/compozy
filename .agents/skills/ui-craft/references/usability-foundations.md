# Usability Foundations

Read this when designing a novel surface, auditing an existing one, or when a usability heuristic violation is suspected. The principles below are the load-bearing filter every UI decision passes through.

## Nielsen's 10 Usability Heuristics

These are the bedrock. They have not changed since 1994 because human cognition has not changed since 1994. Each entry below names the heuristic, the operational test, and the most common violation.

### 1. Visibility of System Status
**Test:** At every moment, can the user tell what the system is doing and what just happened?
**Common violation:** Long-running action with no progress signal; spinner with no message; success quiet, error silent.
**Apply:** Show progress (determinate when possible, indeterminate otherwise). Confirm landed actions transiently. Surface async failures inline, not in a console.

### 2. Match Between System and Real World
**Test:** Does the language match how the user describes the task in their own words?
**Common violation:** Internal jargon in UI (`endpoint`, `payload`, `cluster`), error codes without translation, metric names from the database.
**Apply:** Use the user's vocabulary. Reserve technical names for technical surfaces (logs, API docs).

### 3. User Control and Freedom
**Test:** Can the user back out of any action they started, including ones already committed?
**Common violation:** No undo on destructive actions; no cancel during a multi-step flow; no Esc to dismiss a modal.
**Apply:** Provide explicit exits (Cancel, Close, Esc), undo for reversible actions, confirmation for irreversible ones.

### 4. Consistency and Standards
**Test:** Does the surface behave the same as other surfaces in this product, and like other products in the same category?
**Common violation:** Primary button on the left in one dialog, on the right in another; navigation patterns that contradict the platform.
**Apply:** Internal consistency first (same component, same behavior everywhere). External consistency second (Jakob's Law — users spend most time on other sites).

### 5. Error Prevention
**Test:** Does the design make the slip impossible, or at least require confirmation?
**Common violation:** Destructive action one click from a misclick; free-text input where an enum would suffice; missing constraints.
**Apply:** Constrain inputs with the right control (date picker over date string, dropdown over typo-prone text). Confirm irreversible actions. Validate inline before submit.

### 6. Recognition Rather Than Recall
**Test:** Can the user choose from visible options rather than remember what to type?
**Common violation:** Required-format input without examples; hidden navigation; commands not surfaced in the UI.
**Apply:** Show options. Surface recent items. Provide examples in placeholder/help text. Never require users to remember syntax.

### 7. Flexibility and Efficiency of Use
**Test:** Can experts work faster than novices through shortcuts, batch actions, defaults?
**Common violation:** No keyboard shortcuts; no bulk select; no remembered preferences.
**Apply:** Layer accelerators on top of the basic flow (keyboard shortcuts, bulk select, recent values, saved views). Never make the accelerator the only path.

### 8. Aesthetic and Minimalist Design
**Test:** Every visible element earns its place by serving the user's job here.
**Common violation:** Decorative illustrations on functional pages; secondary information competing with the primary action; multiple "features" surfaced at once.
**Apply:** Subtract before adding. One primary action per view. Decoration is a moment, not a layout.

### 9. Help Users Recognize, Diagnose, and Recover from Errors
**Test:** When something fails, does the message say what happened, why, and what to do?
**Common violation:** `Error 500.`; `Something went wrong.`; `Invalid input.`
**Apply:** Plain language. Specific cause. Concrete recovery. Never blame the user. See `microcopy-quality.md` for the full pattern.

### 10. Help and Documentation
**Test:** Is help in context, task-oriented, and reachable from the surface where it is needed?
**Common violation:** Docs in a different domain, separated from the UI; help that is a wall of text; tooltips that only repeat the label.
**Apply:** Inline help next to the field. Linked deep-docs for procedures. Empty states that teach the feature, not just announce its existence.

## Laws of UX

Cognitive principles that govern how users perceive and interact with interfaces. These are not opinions — they are observed regularities in how humans behave.

### Hick's Law — Decision time scales with options
The more choices and the more visually similar they are, the longer the decision and the higher the abandonment.
**Apply:** Reduce visible options. Group related actions. Hide rarely-used actions behind a `More` affordance. Default the safe choice.

### Jakob's Law — Users prefer your product to work like every other product they use
Familiar conventions are not a creativity ceiling, they are a velocity floor.
**Apply:** Use platform conventions for navigation, forms, dialogs, search. Deviate only where the deviation creates real user value, and budget the learning cost.

### Miller's Law — Working memory holds 7±2 items
For lists, navigation, filters, breadcrumbs, and onboarding steps.
**Apply:** Chunk long lists into groups of 5–7. Break long forms into steps. Surface no more than 7 navigation items at the top level.

### Fitts's Law — Target acquisition time is a function of distance and size
Small targets, far away, are slow and error-prone. Critical for touch surfaces and dense desktop UIs.
**Apply:** Larger targets for primary actions (≥ 44×44 on touch, ≥ 32×32 on desktop). Place destructive actions farther from primary actions. Edges and corners are infinite-size targets (use them for high-frequency tools).

### Tesler's Law (Conservation of Complexity) — Complexity is moved, not removed
For any system, there is an irreducible complexity that must be borne by either the user, the designer, or the system.
**Apply:** Ask which party should carry the complexity. Defaults move complexity from user to designer. Smart parsing moves it from user to system. Never push complexity onto the user that the designer could have absorbed.

### Cognitive Load (Sweller) — Mental resources are finite
Three types: intrinsic (the task itself), extraneous (the interface), germane (learning).
**Apply:** Cut extraneous load mercilessly. Visual noise, redundant labels, decoration without function — all are tax on a finite budget. Spend the budget on the task, not on chrome.

### Aesthetic-Usability Effect (Kurosu & Kashimura) — Beautiful UIs are perceived as more usable
A correctly executed visual system raises user tolerance for friction. The inverse is also true: an ugly UI gets blamed for failures that are actually elsewhere.
**Apply:** Visual craft is not optional. It is part of usability.

### Progressive Disclosure (Krug, NN/g) — Show the core, defer the rare
Two levels max in 95% of cases. More than two and the disclosure mechanism itself becomes a navigation problem.
**Apply:** Primary controls visible by default. Advanced options behind a clearly marked affordance (`Advanced`, `More options`, `Customize`). Tertiary options behind a settings page or contextual menu. Never bury the common-case path.

### Chunking — Grouping items into meaningful wholes increases recall and scanability
**Apply:** Group fields by relationship in forms. Group nav items by domain. Use whitespace to define the group, not borders, unless density demands them.

### Serial Position Effect (Ebbinghaus) — First and last items are remembered best
**Apply:** Put the most important nav item first and the most distinctive last. In a long list, anchor with sticky headers or page markers.

### Zeigarnik Effect — Interrupted tasks are remembered better than completed ones
**Apply:** Show progress on long flows. Persist in-progress state across navigation. Resume where the user left off. Closure (completed-step checkmarks, success states) is part of the design.

### Doherty Threshold — Productivity soars when system response ≤ 400ms
**Apply:** Optimistic UI for ≤ 1s actions. Skeleton or progress for > 1s. Never block the user for > 5s without an explicit progress signal.

### Postel's Law (applied to UI input) — Be liberal in what is accepted, strict in what is produced
**Apply:** Accept `"jan 1, 2026"`, `"01/01/26"`, `"2026-01-01"` and normalize. Produce a single canonical format back to the user. Validation is not gatekeeping.

## Tognazzini's Principles of Interaction Design (selected)

Bruce Tognazzini's principles add operational depth beyond Nielsen's heuristics. The high-leverage ones for AI-era UI:

- **Anticipation** — the interface predicts what the user needs next and surfaces it without being asked. Recent items, smart defaults, related actions.
- **Autonomy** — the user is in control of the environment, including AI features. They can pause, override, undo.
- **Color, but…** — color alone is never the sole carrier of meaning. Always pair with shape, position, or text. Red is not "error" to a user with red-green color blindness; an `X` icon plus the word `Error` plus red is.
- **Defaults** — defaults must be the safe, common, recoverable choice. Defaults are not personality; they are protection.
- **Fitts's Law in practice** — primary actions get more weight (size + contrast); destructive actions get distance from primary actions or a confirmation step.
- **Latency reduction** — perceived latency matters more than actual latency. Skeleton screens, optimistic updates, and progressive rendering buy seconds.
- **Modes are dangerous** — modal states change what controls mean. Modes need strong visual treatment and an unmissable exit.
- **Visible Navigation** — users should always know where they are, where they came from, and where they can go.

## Mental Models

Users do not see the system. They see a model in their head about how the system works. That model is built from their previous experiences, the UI's affordances, and the words it uses.

**Implications:**
- A new UI inherits whatever model the user brings (Jakob's Law). Conform or pay the learning cost.
- Words shape the model. Calling a thing `workflow` vs `pipeline` changes how the user reasons about it.
- The model can be wrong. When it is, the interface is failing — find the misalignment, fix the words, the structure, or the behavior.
- Show the model. Diagrams, breadcrumbs, scoping indicators help the user see the structure they otherwise have to infer.

## The Operational Filter

When evaluating a surface, run it through this filter in order. Stop at the first failure:

1. **Status visible?** Can the user see what the system is doing?
2. **Language matched?** Does it speak the user's words?
3. **Exit available?** Can the user back out?
4. **Consistent?** Does it match the rest of the product?
5. **Errors prevented?** Are slips made impossible or confirmed?
6. **Recognition over recall?** Are options visible, not memorized?
7. **Efficient?** Can experts move faster?
8. **Minimal?** Has every element earned its place?
9. **Recovery clear?** When things fail, is the path forward obvious?
10. **Help in context?** Is help where it is needed, not in a faraway doc site?

A surface that passes all ten is not necessarily great. A surface that fails any of them is not yet shippable.

## Sources

- Nielsen Norman Group — 10 Usability Heuristics for User Interface Design (foundational, 1994; updated 2020+)
- Lawsofux.com — Curated cognitive principles for UX (Yablonski)
- Tognazzini — First Principles of Interaction Design (Revised & Expanded)
- NN/g — Progressive Disclosure
