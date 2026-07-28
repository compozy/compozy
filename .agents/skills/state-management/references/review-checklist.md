# Review Checklist

Use this reference when the user asks for a review, refactor guidance, or diagnosis of messy state code.

## Findings to look for

- redundant state that can be derived
- duplicated source of truth across store, component, and URL
- impossible state combinations
- many booleans representing one mode
- props mirrored into local state
- effects that synchronize state rather than react to external systems
- selected object and selected ID both stored
- local UI state pushed global without a clear ownership reason
- relational data stored in shapes that make updates inconsistent
- async lifecycle modeled as disconnected flags
- setters exposed everywhere without event or transition boundaries

## Good review output

Lead with:

1. the invalid or risky state shapes
2. the concrete bug or regression each shape can produce
3. the smaller state model that removes the risk

Then map that model to the current stack.

## Refactor order

When suggesting a cleanup, prefer this order:

1. remove duplicated and derived state
2. collapse booleans into explicit status
3. separate server, client, URL, and ephemeral concerns
4. move derivations into selectors/getters/computed values
5. tighten actions and transitions

That order usually improves clarity faster than introducing a new library.
