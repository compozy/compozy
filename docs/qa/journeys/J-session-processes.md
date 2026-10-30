# J-session-processes: Clean up work a session leaves running

Théo starts a development server and background command, checks their ports and ownership, then
chooses whether they should outlive the session. After keeping them, he restarts the daemon and
cleans them up from **Processes left running**. Independent CLI/API reads confirm every mutation.

1. Start the commands through an agent session and inspect **Processes**.
2. Stop the session with terminate and verify process exit and closed port.
3. Start another session and stop with keep.
4. Restart the isolated daemon; locate the same identities in the orphan aggregate.
5. Terminate those identities and verify the empty aggregate and closed port.

The adjacent canary is a native terminal: shutdown closes it. Windows reports unsupported.
Scenario: `RT-session-processes`; charter: `CH-session-processes`.
