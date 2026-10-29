# Project archive and missing-folder recovery

Projects can now be archived and restored without changing their identity or deleting sessions,
history, layouts, worktrees, or configuration. Archive is refused while work is active; the dialog
identifies sessions, Loops, and enabled automations that must be stopped or disabled first.

Previously, listing projects automatically removed a registration and its stopped history when
its folder was unavailable. A disconnected drive or temporarily moved repository could therefore
cause silent data loss. Missing folders now keep their registration and history and remain visible
in the picker and Settings with **Folder not found** (`root_missing: true`). The flag is computed
from the current filesystem state, not persisted. Selecting the project still permits history
reads; starting a session returns `workspace_root_missing` until its folder is restored. Archive
and Remove remain available. Restoring the folder clears the warning with the same project ID.

Only explicit **Remove project** deletes the registration and stopped session history. Its
confirmation explains the loss, leaves files on disk untouched, and offers **Archive instead**.
The new metadata and commands are additive; no existing API or CLI surface is removed.
