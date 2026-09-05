# Rebase Troubleshooting

- **Conflict:** inspect the current replayed commit and both versions, edit the intended result, stage only resolved paths, and continue.
- **Cannot lock ref:** identify the owning Git process, ref state, and permissions. Wait for legitimate concurrent work; do not kill arbitrary Git processes or delete lock files based only on age.
- **Waiting for an editor:** inspect the editor process/configuration and complete that interaction. A one-command editor override is preferable to changing global configuration.
- **Diverged branch:** expected after rewriting local history, but publication still requires authorization and a lease against the observed remote head.
- **Recovery:** inspect the backup ref and reflog, preserve the current work, and determine what is missing before choosing a recovery action. Do not reset, checkout, stash, or discard work without the required explicit permission.

Diagnose the actual failure and retry the affected operation; do not restart the entire rebase or switch to a merge merely because one operation failed.
