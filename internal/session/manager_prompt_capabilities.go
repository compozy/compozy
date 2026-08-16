package session

import "context"

func (m *Manager) preflightAdmittedPromptAttachments(
	ctx context.Context,
	session *Session,
	runtime *RuntimeSelection,
	attachments []AttachmentMeta,
) error {
	if len(attachments) == 0 || session.IsPrompting() {
		return nil
	}
	proc, err := m.reservePromptSlot(ctx, session, runtime)
	if err != nil {
		return err
	}
	defer session.finishPromptSetup()
	return validatePromptAttachmentCaps(attachments, proc.CapsSnapshot())
}
