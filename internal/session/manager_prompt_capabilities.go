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
	proc, err := session.beginExclusivePromptSetup()
	if err != nil {
		return err
	}
	defer session.finishPromptSetup()
	if err := m.validateReservedRuntimeModel(session, proc, runtime); err != nil {
		return err
	}
	proc, err = m.ensurePromptRuntime(ctx, session, runtime, proc)
	if err != nil {
		return err
	}
	return validatePromptAttachmentCaps(attachments, proc.CapsSnapshot())
}
