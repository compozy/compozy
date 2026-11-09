## 0.5.5 - 2026-11-09

### Bug Fixes

- Keep SQLite statement journals in memory during write transactions (#1367)
- Hide automation-run sessions from the session list (#1366)
- Make TestSetProcessGroupNiceness deterministic and document the setpriority fork race (#1373)
- Keep session owner snapshots valid while the writer appends WAL frames (#1371)
- Let triage cleanup wait for the full stop ladder before archiving (#1365)
- Keep internal daemon handoff variables out of session and terminal environments (#1364)
- Prevent source executable contention and isolate cooldown fixtures (#1375)
- Keep the Files identity in its window head and route transcript paths to Files tabs (#1378)
- Return main CI to green after the 2026-11-08 merges (#1380)
- Apply Linux launch niceness before exec (#1386)
- Preserve prompt completion across concurrent steering events (#1376)
- Move core WAL restart synchronization to maintenance (#1377)
- Settle automation fires after agent startup failures (#1389)
- Preserve explicitly empty environments through process launch (#1390)
- Wait for archive responses before retiring session windows (#1392)
- Name automation and always-on sessions by origin and keep relative times live (#1391)
- Clear acknowledged always-on agent attention (#1395)
- Retain lifecycle ownership across startup and shutdown races (#1394)
- Recover acknowledged attention without blocking live sessions (#1396)
- Retain startup recovery through process and storage failures (#1398)
- Preserve attention publication ordering and lifetimes (#1401)
- Preserve failed startup event sequences and cleanup ownership (#1404)
- Bind approvals to run identity and explain terminal denials (#1412)

### Features

- Browse the CompozyOS marketplace by category with one-step Add and a curated plugin catalog (#1360)
- Let always-on check-ins call the agent's tools and auto-accept declared proposals (#1385)

### Testing

- Repair check-in proposal pick runtime fixtures (#1406)

### Release Notes

#### Fixes

##### Automation runs stay out of the session list

Sessions started by an automation job or trigger no longer show up in the session list, the board or their counts, and a new run no longer takes the selection. Before, a scheduled run appeared at the top of the list as "New session" with a working spinner while it ran. Each run stays reachable from the Automations run view and through search, and a run that needs you still reaches you through attention toasts and notifications.

To see them, turn on **Show automation runs** in the list's or board's **View options**. Each window remembers the choice. A run you prompted yourself stays on the list. Loop and Task sessions are unchanged. The dock's Sessions launcher now opens your newest own session, never an automation run.

Migration notes: no action needed; existing runs follow the same rule after upgrading. `compozy session list` and `compozy__session_list` keep returning every session. The API does too, unless a client passes the new `exclude_automation=true` query parameter on `GET /api/sessions` or `GET /api/sessions/facets`. `exclude_finished_automation=true` keeps its meaning.
