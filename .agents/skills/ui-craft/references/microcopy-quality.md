# Microcopy Quality

Read this when writing or reviewing any user-visible text. Microcopy is design — it shapes mental models, sets tone, and decides whether the user understands what just happened. Bad microcopy is the most common AI-slop tell.

## The Core Rules

### 1. Every word earns its place
Cut filler ruthlessly. Compare:

- ❌ `Please click the button below to continue.`
- ✅ `Continue`

- ❌ `Note that you will need to verify your email before proceeding.`
- ✅ `Verify your email to continue.`

- ❌ `In order to delete this project, you must first confirm.`
- ✅ `Delete this project?`

### 2. Verb-first for actions
CTA buttons and action links lead with the verb:

- ❌ `OK` / `Submit` / `Continue` / `Click here`
- ✅ `Delete project` / `Save changes` / `Send invite` / `Start workflow`

The verb names what happens; the noun names what it happens to. Two-word CTAs beat one-word CTAs almost always.

### 3. Specific over generic
Generic copy is a tax on user attention. Specific copy is design.

- ❌ `Are you sure?`
- ✅ `Delete this project and all its data?`

- ❌ `Invalid input.`
- ✅ `Email must include @ and a domain.`

- ❌ `Something went wrong.`
- ✅ `Couldn't save — your session expired. Sign in again to continue.`

### 4. Plain language, no jargon
Match the user's vocabulary, not the database's:

- ❌ `Workflow execution failed with status 500.`
- ✅ `Workflow didn't finish — the server stopped responding.`

- ❌ `Authentication token expired.`
- ✅ `Your sign-in expired — sign in again.`

- ❌ `Invalid payload schema.`
- ✅ `The form has 3 fields with errors. Fix them and try again.`

Technical terms are appropriate for technical surfaces (logs, API docs, dev tools). Reserve them for the right audience.

### 5. Plain English for errors
The user-facing error format:
- **What happened:** plain, factual, no blame
- **Why (when knowable):** brief context
- **What to do:** the concrete next step

Example:
> Couldn't sync your changes — you're offline. We'll retry when you're back online.

### 6. Brevity without sacrificing clarity
Short is the goal, not the rule. Cut filler but keep the cues that prevent confusion.

- ❌ `Saved.` (cryptic if multiple things could have saved)
- ✅ `Project settings saved.`

- ❌ `Member added to all projects in your organization successfully.` (overstuffed)
- ✅ `Added member to all projects.`

### 7. Tone matches stakes
- **Routine actions:** neutral, direct (`Save`, `Add tag`, `Open project`)
- **Destructive actions:** plain, no hedging (`Delete project`, `Remove member`, `Revoke access`)
- **Errors:** factual, no blame, no apology cascade (`Couldn't save` — not `Oops! Something went terribly wrong! We're so sorry!`)
- **Success on routine action:** transient confirmation, no fanfare (`Saved`)
- **Success on rare action:** can celebrate with restraint (`Workflow published — visible to your team`)

### 8. Never blame the user
- ❌ `You entered an invalid email.`
- ✅ `Email must include @ and a domain.`

- ❌ `You forgot to fill in this required field.`
- ✅ `This field is required.`

Errors are about the system's expectations, not the user's failures.

### 9. Don't perform delight
"Delight" copy that the user didn't ask for is friction:

- ❌ `Welcome back! We're so excited to see you again! 🎉`
- ✅ (no greeting at all — drop the user into the work)

- ❌ `Loading your awesome dashboard...`
- ✅ (skeleton screen, no copy)

- ❌ `Yay! All done! 🎊`
- ✅ `Saved.`

### 10. Confirm with the verb of the action
Confirmation dialogs use the same verb as the trigger:

- Trigger button: `Delete project`
- Dialog primary action: `Delete project` (NOT `Yes` / `Confirm` / `OK`)

This makes the user's choice unambiguous when scanned out of context.

## The Banned AI Vocabulary

These words and phrases are LLM tells. Strip them from product copy ruthlessly.

### Verbs to avoid
`elevate`, `empower`, `unleash`, `harness`, `unlock`, `revolutionize`, `transform` (overused), `delve`, `leverage` (in marketing voice), `streamline`, `supercharge`, `craft` (as in "craft an experience"), `curate` (overused)

### Nouns to avoid
`journey`, `experience` (as marketing fluff), `tapestry`, `landscape`, `ecosystem` (overused), `paradigm`, `solution` (when "tool" or "feature" would do), `synergy`

### Adjectives to avoid
`seamless`, `intuitive` (claim, not description), `cutting-edge`, `state-of-the-art`, `robust` (often means nothing), `comprehensive` (often means nothing), `holistic`, `vibrant`, `dynamic`

### Phrases to avoid
- `take it to the next level`
- `unlock the power of`
- `at your fingertips`
- `your one-stop shop for`
- `the future of X is here`
- `we're so excited to`
- `welcome to the future of`
- `revolutionize your workflow`
- `delve into`
- `in today's fast-paced world`

### Greetings to avoid
- `Welcome!`
- `Welcome back!`
- `Hi there!`
- `Let's get started!`
- `Hello, friend!`

Most product surfaces need no greeting at all. The user came to do a thing — let them do it.

### Em-dash overuse
Em-dashes are not banned, but they are a classic AI tell when used 3+ times in a short block. Use them sparingly and only when a comma or period would not work.

### Decorative rule of three
LLMs love three-item lists for cadence. When the third item is filler, cut it:

- ❌ `Build, ship, and iterate faster.` (if "iterate" is just rhythm padding)
- ✅ `Build and ship faster.`

## Per-Surface Tone Guides

### Buttons
- Imperative verb + noun. 1–3 words.
- Sentence case (`Save changes`), not Title Case
- No periods, no exclamation marks

### Links
- Describe the destination, not the action
- ❌ `Click here for docs`
- ✅ `Read the API reference`
- Never use the URL as link text in body copy

### Page titles
- Name the surface in user vocabulary
- `Projects`, `Workflow editor`, `Team settings`
- Never `Manage Your Projects` (Title Case + filler verb)

### Section headings
- Describe what is inside
- `Recent runs` (not `Your Recent Runs`)
- `Members` (not `Manage Members`)

### Labels
- Singular noun, describes the field
- `Name`, `Description`, `Email`, `Tags`
- Not `Please enter your name`

### Helper text (under labels)
- Only when the label is ambiguous
- Names the format or constraint
- `Use lowercase letters and hyphens` / `Maximum 80 characters`
- Never repeats the label

### Placeholders
- An example of valid input, not a label substitute
- `acme-corp` / `jan@example.com` / `Project name`
- Distinguish placeholder text visually (lower contrast, italic) to avoid confusion with user input

### Error messages
- See "The Core Rules" above
- Pattern: `<plain description of what happened>. <how to fix>.`
- 1–2 sentences

### Empty states
- Name the data type (`No projects yet`)
- Explain what lives here (`Projects organize your workflows and runs.`)
- The single primary action (`Create your first project`)

### Confirmation dialogs
- Title = the question or decision (`Delete this project?`)
- Body = the consequence (`All workflows, runs, and logs will be permanently removed.`)
- Primary action = the verb (`Delete project`)
- Secondary = `Cancel`

### Tooltips
- 1 sentence max, ≤ 80 characters
- Plain text, no formatting
- Names what the icon means or what the action does

### Toasts
- 1 short sentence
- Verb-led where applicable (`Saved`, `Copied`, `Couldn't connect`)
- Optional action: `Undo`, `View`, `Retry`

### Loading and skeleton text
- Usually no text — the skeleton is the message
- If text needed: `Loading…` or surface-specific (`Searching workflows…`, `Saving…`)
- Never `Loading awesome content…` or any decorated variant

## Required vs Optional Marking

When most fields are required:
- Do not mark required with `*` (adds noise)
- Mark optional fields with `(optional)` next to the label

When most fields are optional:
- Mark required with `*` and explain `* required` near the form

Consistency within a product matters more than which pattern.

## Numbers, Dates, Units

- Use locale-aware formatting (`Intl.DateTimeFormat`, `Intl.NumberFormat`)
- Relative time for recent events (`5 minutes ago`, `Yesterday`)
- Absolute time for distant events (`May 12, 2026`)
- Always show timezone for scheduled events (`2:00 PM PT`)
- Units always separated by space (`100 ms`, `1.4 GB`)
- Pluralize correctly (`1 project`, `2 projects` — not `1 projects`)

## Conversational Microcopy

Sometimes conversational tone serves the user. Use it when:
- The product persona is friendly (consumer apps, education)
- The surface is helping the user out of a hole (empty states, first-run)
- The message is celebratory and earned (first workflow shipped, milestone reached)

Avoid conversational tone when:
- Stakes are high (destructive confirmations, errors)
- The user is in a focused work mode (editor chrome, table headers)
- The product persona is professional (enterprise tools, admin panels)

A consistent voice across the product matters more than maximizing friendliness.

## A Quick Audit Checklist

Run any block of copy through this:

- [ ] Each word earns its place (cut filler)
- [ ] Verbs are specific and first in CTAs
- [ ] No banned AI vocabulary (see list above)
- [ ] No unwanted greetings or "delight" decoration
- [ ] Errors say what happened, why, what to do
- [ ] No blame on the user
- [ ] Tone matches the stakes
- [ ] Reads like a thoughtful human wrote it, not an AI on autopilot

## Sources

- NN/g — Error message guidelines, content design heuristics
- UK Government Design System — Style guide (plain English, error messages, content patterns)
- Polaris (Shopify) — Content guidelines (voice, tone, terminology)
- Wikipedia — Signs of AI writing (banned vocabulary reference)
- Mailchimp Content Style Guide — Voice, tone, microcopy patterns
