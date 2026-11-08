package session

import "context"

// preflightAdmittedPromptAttachments joins startup ownership before validating a Goal runtime binding.
func (m *Manager) preflightAdmittedPromptAttachments(
	ctx context.Context,
	session *Session,
	runtime *RuntimeSelection,
	attachments []AttachmentMeta,
) error {
	if len(attachments) == 0 || session.IsPrompting() {
		return nil
	}
	// Match ordinary prompt admission: startup owns its binding until its run settles.
	if start := m.sessionStartRun(session.ID); start != nil && !start.activationCommitted.Load() {
		if err := waitForSessionStartRun(ctx, start); err != nil {
			return err
		}
	}
	proc, err := m.reservePromptSlot(ctx, session, runtime)
	if err != nil {
		return err
	}
	defer session.finishPromptSetup()
	return validatePromptAttachmentCaps(attachments, proc.CapsSnapshot())
}
