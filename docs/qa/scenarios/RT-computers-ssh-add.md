---
id: RT-computers-ssh-add
area: RT
title: Set up a remote computer over SSH
persona: Iris
journey: J-operate-remote-computers
expected: The controller verifies the release and host key, installs a persistent service, and connects over SSH.
entry_points: compozy computer add ssh, compozy computer setup, compozy service, Settings → Computers
qa_status: pending
---

1. Set up a fresh Linux VM with a user systemd session. Add its alias and `StrictHostKeyChecking accept-new` to the controller's SSH config. Record the VM's fingerprint independently.
2. Start SSH setup and decline the fingerprint. Confirm the setup stops, the Free slot is available, and no runtime or service was written to the VM. Repeat and accept the correct fingerprint. Verify release, install, service, link, and forward progress.
3. Verify `compozy service status --json` on the VM and `compozy computer status <name> --json` on the controller. Run `compozy session new --computer <name> --workspace '<absolute path on the VM>' --agent general` with a workspace registered on the VM. Send a prompt with `compozy session prompt <session_id> --computer <name> "<message>"`. Quit the controller as soon as the command confirms acceptance. Restart the controller and use `compozy session history <session_id> --computer <name> --json` to verify that the reply was recorded after the controller quit.
4. Interrupt a second setup before linking. Repeat the add command and verify it resumes at the first incomplete step. Cancel it and verify the slot becomes available before another add proceeds.
5. Rotate a trusted VM host key and verify the controller reports `ssh_host_key_changed`. Confirm the previously trusted key stays pinned.

Browser coverage: E2E-002 exercises host-key trust, the SSH connection, and a session started on the new computer through Run on. E2E-003 covers adding an account-linked computer and finding one of its sessions in the sidebar and Search.

Walk this scenario after the Remote Computers slices are integrated.
