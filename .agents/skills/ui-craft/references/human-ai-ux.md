# Human-AI UX

Read this when building agent or AI-feature interfaces — chat, streaming responses, autocomplete, generative actions, AI-assisted workflows, copilots, recommendations. AI in product UI is a new contract with the user and most products handle it badly. The rules below are the floor.

## The Core Tension

AI features are nondeterministic, can hallucinate, can be slow, and can produce subtly wrong output that looks confident. Classic UI patterns assume deterministic, fast, correct systems. The gap between those assumptions and AI reality is where slop lives.

The goal of AI UX is to make the nondeterminism legible, the failure recoverable, and the trust earned.

## The 18 Guidelines (Microsoft Human-AI Interaction)

Microsoft Research's 18 guidelines organize into 4 phases of interaction. Treat these as the canon.

### Phase 1 — Initial Interaction (G1–G2)

**G1. Make clear what the system can do.**
- Surface AI capabilities up front. Set expectations.
- Example: a "What can this assistant help with?" intro, not a blinking cursor.

**G2. Make clear how well the system can do what it can do.**
- Acknowledge limitations explicitly.
- Example: "This is an experimental feature — answers may be wrong. Verify critical information."

### Phase 2 — Regular Interaction (G3–G11)

**G3. Time services based on context.**
- Don't interrupt with suggestions during high-focus work
- Time generation to user pauses or explicit requests

**G4. Show contextually relevant information.**
- Surface information that matches what the user is currently doing
- Avoid generic suggestions in specific contexts

**G5. Match relevant social norms.**
- Use language and behavior appropriate to the user's culture and context
- Don't perform familiarity the relationship hasn't earned

**G6. Mitigate social biases.**
- Audit AI outputs for biased language and assumptions
- Provide ways to flag biased output

**G7. Support efficient invocation.**
- The AI feature should be easy to trigger when needed
- Keyboard shortcut, predictable location, low friction

**G8. Support efficient dismissal.**
- The AI feature should be easy to dismiss when not needed
- Esc, click-outside, X button — predictable

**G9. Support efficient correction.**
- Make it easy to refine, regenerate, or correct AI output
- One-click "try again," editable output, "make it shorter" affordances

**G10. Scope services when in doubt.**
- When uncertain, narrow the AI's response or ask a clarifying question
- Better to ask "Did you mean X or Y?" than to confidently answer the wrong thing

**G11. Make clear why the system did what it did.**
- Explain reasoning, cite sources, show retrieval results
- "I used these documents" + clickable links

### Phase 3 — When Wrong (G12–G14)

**G12. Remember recent interactions.**
- Carry context across the conversation
- Don't make the user repeat themselves

**G13. Learn from user behavior.**
- Improve over time based on actions taken (with consent)
- Surface this learning to the user when relevant

**G14. Update and adapt cautiously.**
- Behavior changes should be communicated, not silent
- Updates that change AI behavior need release notes

### Phase 4 — Over Time (G15–G18)

**G15. Encourage granular feedback.**
- Per-response thumbs-up/down, "Why was this wrong?" prompt
- Free-text feedback for nuance

**G16. Convey the consequences of user actions.**
- "Accept this AI edit" — explain what changes
- "Use this AI-generated code" — explain where it goes

**G17. Provide global controls.**
- User can turn AI features off entirely
- User can set tone, response length, persona

**G18. Notify users about changes.**
- Model updates, capability changes, deprecations
- Especially for changes that affect output quality or behavior

## IBM Design for AI (Relational Model)

IBM frames AI UX as designing the *relationship* between user and system. Four pillars:

### 1. Accountability
The system takes responsibility for its outputs. When wrong, it acknowledges. When unsure, it says so.

**Apply:**
- Never present uncertain output as confident
- Surface confidence levels when meaningful
- Provide audit trails for decisions

### 2. Value Alignment
The system advances the user's goals, not the operator's. Recommendations serve the user's interest.

**Apply:**
- Surface conflicts of interest (sponsored, ranked, sorted by)
- Let the user override default values
- Default to user-favoring choices

### 3. Explainability
The system can explain *why* — the inputs, the reasoning, the sources.

**Apply:**
- Cite sources for factual claims
- Show retrieval results for RAG-based answers
- Explain why a suggestion is being made
- Provide "How did you get this?" affordance

### 4. User Data Rights
The user owns their data and conversation history. Export, delete, redact.

**Apply:**
- Show what data the AI has access to
- Provide controls to clear context, delete history
- Be explicit about training-data usage

## Streaming UI

Streamed responses (token-by-token output) need their own design vocabulary.

### Affordances
- **Visible cursor** during generation — signals "still working"
- **Stop button** — first-class affordance to abort generation
- **Regenerate** — after completion, offer to try again
- **Edit prompt and resubmit** — let the user refine their input without retyping

### Pacing
- First token within 1–2 seconds (or show "Thinking..." state)
- Subsequent tokens at human reading pace (40–80ms per token)
- Final pass: minor adjustments after streaming complete (newlines, formatting)

### Layout stability
- Reserve space for the full response area
- Avoid scroll-jumping as content grows
- Auto-scroll to follow the cursor only when the user is at the bottom

### Errors mid-stream
- Show the partial output with a clear marker (`Generation interrupted`)
- Offer to continue, regenerate, or copy what was generated
- Never silently truncate

## Citations and Sources

When the AI claims a fact, provide the source.

### Inline citations
- Superscript number or footnote-style reference
- Click to expand: source title, snippet, link
- Multiple sources for the same claim: list all

### Source panel
- Sidebar listing all sources used in the response
- Confidence indicator per source (when available)
- Removed sources marked clearly (`source no longer accessible`)

### Patterns
- Perplexity-style numbered citations work for general Q&A
- Inline highlights work for document-grounded RAG (highlight the source passage)
- "Show your work" affordance reveals retrieval results

## Confidence and Uncertainty

When the model is uncertain, the UI should reflect that.

### Display patterns
- "Likely" vs "Possibly" vs "I'm not sure" — graded language
- Confidence scores when meaningful (and explainable)
- Visual treatment for low-confidence answers (subtle background, label, icon)

### Don'ts
- Don't fake confidence scores when not meaningful
- Don't make confidence numeric when it's actually categorical
- Don't show confidence as a single value when multiple parts of the answer have different confidence

## Hallucination Containment

Some outputs are higher-stakes than others. Hallucination tolerance varies.

### High-stakes (low tolerance)
- Medical, legal, financial advice
- Code that will be executed
- Data displayed as authoritative

**Pattern:** Constrain output to retrieved sources (RAG), refuse to answer when unsure, provide clear "verify with a professional" affordances.

### Medium-stakes
- Suggestions, summaries, draft text
- Search results, recommendations

**Pattern:** Cite sources, allow easy verification, mark output as AI-generated.

### Low-stakes
- Creative tasks, brainstorming, exploration
- Casual conversation

**Pattern:** Standard streaming UI, easy correction and regeneration.

## Agent UI Patterns

Agents — LLMs with tools, memory, and autonomy — need richer UI than pure chat.

### Action surfacing
- Show what the agent is doing in real time ("Searching docs", "Querying database", "Running script")
- Reveal tool calls in a collapsible "thinking" panel
- Allow inspection of intermediate results

### Approval gates
- Destructive or expensive actions: require explicit user approval
- Show preview of the proposed action
- "Approve this run" or "Edit and approve"

### Long-running tasks
- Progress indicator with current step
- Cancelable at any step
- Resumable on disconnection
- Result persisted, not just streamed

### Memory transparency
- Surface what the agent remembers about the user
- Provide controls to edit, delete, or reset memory
- Distinguish session memory from persistent memory

## Prompt Surfaces

When the user types a prompt, the input is the most important interactive element on the surface.

### Layout
- Large enough text area (multi-line by default, expandable)
- Visible keyboard shortcut hint (Cmd+Enter to send, Shift+Enter for newline)
- Send button visible but secondary to the keyboard shortcut
- Attach affordance (files, images) inline with the input

### Affordances
- Slash commands or @-mentions for structured input
- Suggestion chips for common starting prompts
- History (↑ arrow to recall previous prompts)
- Persistent draft (don't lose input on accidental navigation)

### After submission
- Lock the input briefly to prevent double-submit
- Clear or persist based on UX choice (chat → clear; query → persist)
- Move the prompt into the conversation history

## Error Recovery

AI errors are different from classic errors. They're often probabilistic, not deterministic.

### Patterns
- **Rate limit:** "I'm at my limit for now. Try again in N seconds." + countdown
- **Model error:** "Couldn't generate a response. Try rephrasing, or try a shorter prompt."
- **Content filter:** "I can't help with that. Here's why: [policy reason]. Try [alternative]."
- **Network failure:** "Connection lost. Your draft is saved. Reconnect and retry."
- **Context too long:** "Conversation is too long for me to track. Start a new one, or I'll summarize and continue."

### Always offer
- Retry (one click)
- Regenerate (try a different output)
- Edit prompt and resubmit
- Copy partial output (if any)
- Start over

## Trust-Building Anti-Patterns

Things that erode user trust in AI features:

- **Performing confidence on uncertain output** — looks competent, hurts long-term trust
- **Generic apologies after every error** — "I'm so sorry!" rings hollow at scale
- **Excessive enthusiasm** — "Great question! Let me dive deep into this fascinating topic!"
- **Hiding tool use** — pretending the LLM "knows" something when it actually retrieved it
- **Personifying the AI inconsistently** — "I" sometimes, "the assistant" other times
- **Sudden behavior changes** — the AI was helpful yesterday and refuses today, with no explanation

## Microcopy for AI Features

Voice rules for AI UI text (compounding `microcopy-quality.md`):

- **Use plain "I" or no pronoun** — consistent. Avoid "the AI", "the assistant", "the model" mixed inside one product.
- **Acknowledge limits without groveling** — "I'm not sure" beats "I'm so sorry, I'm just an AI and I can't be certain..."
- **Don't perform excitement** — "Great question!" / "What an interesting prompt!" are banned
- **Name what failed** — "Couldn't reach the search index" beats "Something went wrong"
- **No filler thinking** — "Let me think about this..." with no actual delay is dishonest; either show real progress or skip it

## A Verification Loop for AI Features

Before shipping an AI feature, run this:

1. **Can the user tell what it does?** (G1)
2. **Can the user tell how well it does it?** (G2)
3. **Can the user trigger and dismiss it easily?** (G7, G8)
4. **Can the user correct or refine output?** (G9)
5. **Are sources or reasoning surfaced?** (G11)
6. **Is uncertainty visible?**
7. **What happens when it's wrong?** (Recovery flow tested)
8. **Can the user turn it off?** (G17)
9. **Does the user know when behavior changes?** (G18)
10. **Do the standard UI floors still hold?** (a11y, contrast, keyboard, motion)

If any answer is "no," the feature is not shippable yet.

## Sources

- Microsoft Research — Guidelines for Human-AI Interaction (Amershi et al., CHI 2019)
- IBM Design for AI — Fundamentals, Ethics, Conversation Design
- Anthropic — Claude prompting best practices (for surface design intuitions)
- Martin Fowler — Patterns for Generative AI Application Design
- Apple Design Resources — Designing for Apple Intelligence (HIG)
- Google — People + AI Guidebook
