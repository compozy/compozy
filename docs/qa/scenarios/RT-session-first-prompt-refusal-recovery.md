---
id: RT-session-first-prompt-refusal-recovery
area: RT
title: A first prompt with a skill and attachments survives a mid-bind reload, and a refused send is never silent
persona: Théo
journey: J-session-attachments
expected: A first prompt that invokes a skill and carries attachments is admitted even when a registry reload (such as a profile-selection change) lands during the runtime bind. A prompt the daemon does refuse after admission leaves a `prompt_dropped` transcript marker (code `prompt_dispatch_indeterminate`) and a daemon WARN log line. The web shows "Message not sent: {reason}. Your draft was kept." and returns the exact text and files to that session's composer, even after the session window closed mid-request. Enter, like the disabled Send control, is held while a file is still saving and answers with the blocker reason.
entry_points: web session composer; HTTP prompt route; PUT /api/profiles/selection; compozy logs; session events
qa_status: pass
bug_ids: GH-815
fix_status: fixed
retest_status: pass
fix_commits: current-pr-head
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-815/02-refusal-toast-after-window-closed.png; https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-815/03-draft-and-attachment-restored.png; https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-815/04-enter-held-while-saving.png
last_report: GH-815
overlaps: RT-session-prompt-idempotency; ET-session-composer-skill-chip; ET-session-attachment-picker
---

Use an isolated lab with a workspace skill under `.agents/skills/`. Create a session on one provider and select another provider with `PUT …/runtime`, so the first prompt has to bind a runtime. Send the first prompt with a `/skill` token and an image, with a `message_id` and `idempotency_key`. About 0.7 s into the bind, send `PUT /api/profiles/selection` (`scope=global`). The prompt must answer 200. The persisted `user_message` must carry `skill_invocations`, and the agent must follow the skill. The command catalog's `source.key` for the skill must stay identical before and after the profile switch.

For the refusal path, delete the skill directory right after sending, so expansion legitimately fails after admission. While the request is in flight, close the session window. The web must show the refusal toast with the daemon's reason. Reopening the session from the sessions list, without a page reload, must show the original text, the skill chip and the attachment tile in the composer. Session events must contain one `transcript_marker.created` with `transcript_marker.prompt_dropped` and `prompt_dispatch_indeterminate`, and the daemon log must contain `session: prompt dispatch indeterminate`.

For Enter, delay the attachment upload, type text, attach a file and press Enter while the tile still reads "Saving…". A "Saving {file}" warning must appear, the composer text must stay, and no prompt POST may leave.

2026-10-19 isolated walk (daemon and Web built from the fix branch, served through `COMPOZY_WEB_DIST_DIR`):

- Before the fix, on a v0.3.0 build, the prompt plus profile switch returned `409 prompt_dispatch_indeterminate … expand invoked skills: command: unavailable: /lab-orchestrate` in 2.4 s. Only `hook.dispatch.*` events were persisted, and there was no log line. The catalog key moved from `…@generation:6` to `…@generation:7`.
- After the fix, three runs of the same race each returned 200. The key stayed `root_<digest>` before and after the switch, and the agent replied `ORCHESTRATED`, as only the skill instructs.
- The refusal walk produced the toast and restored the draft and attachment (screenshots above). Session events held the `prompt_dropped` marker, and the daemon log held the WARN line with `message_id` and `turn_id`.
- The Enter walk showed "Saving screenshot-attachment.png" with zero prompt POSTs.
