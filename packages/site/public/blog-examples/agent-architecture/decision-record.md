# Agent architecture decision

## One observed failure

- Input / task identifier:
- Expected behavior:
- Actual behavior:
- Evidence location (trace, issue, external record):

## Failing boundary

- Last confirmed event:
- First uncertain or incorrect event:
- System that owns each event:
- Is this output quality, control flow, persistence, process operation, or an external effect?

## State that must survive

- Proposal / artifact identity and immutable revision:
- Approval and the exact content it authorizes:
- External action identity and reconciliation method:
- Which records are authoritative? Which are cached observations?

## Smallest change

- Proposed change:
- Existing components to retain:
- New operational responsibilities:
- Alternatives rejected, with reasons:
- Assumptions that would invalidate this decision:

## Failure drill

Use disposable inputs. Record both local and receiver state.

| Interruption                                    | Expected recovery | Observed recovery | Evidence |
| ----------------------------------------------- | ----------------- | ----------------- | -------- |
| After draft persistence                         |                   |                   |          |
| While awaiting approval                         |                   |                   |          |
| After external acceptance, before local receipt |                   |                   |          |
| After editing an approved artifact              |                   |                   |          |

## Release and revisit

- Acceptance criteria:
- Remaining uncertainty and its operator:
- Conditions that justify a larger migration:
- Owner and decision date:
