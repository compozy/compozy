# Glossary Format

Store the workspace's ubiquitous language at `.compozy/GLOSSARY.md`. Create the file lazily, only when the first glossary update is accepted. The glossary is free-form and unvalidated.

## Structure

```md
# Glossary

**Order**: A customer's confirmed request for one or more products.
_Avoid_: Purchase, transaction

**Invoice**: A request for payment sent to a customer after delivery.
_Avoid_: Bill, payment request

**Customer**: A person or organization that places orders.
_Avoid_: Client, buyer, account
```

Use `**Term**:` followed by a one-to-two-sentence definition. Add an optional `_Avoid_:` line with comma-separated aliases when the project needs one canonical term.

## Rules

- **Be opinionated.** When multiple words name the same concept, pick the best one and list the others under `_Avoid_:`.
- **Keep definitions tight.** Use one or two sentences. Define what the concept is, not its implementation.
- **Keep only project-specific domain terms.** Exclude general programming concepts even when the project uses them extensively.
- **Prevent duplicates.** Match terms and `_Avoid_:` aliases case-insensitively. Sharpen an existing entry instead of appending the same concept again.
- **Group terms only when useful.** Add descriptive subheadings when natural clusters emerge; keep a flat list when they do not.
- **Preserve declined state.** If the user declines a proposed entry or revision, leave the glossary byte-for-byte unchanged.
