package hooks

import "context"

// DispatchInputPreSubmit runs the input.pre_submit hook pipeline.
func (h *Hooks) DispatchInputPreSubmit(
	ctx context.Context,
	payload InputPreSubmitPayload,
) (InputPreSubmitPayload, error) {
	return h.executeDispatch(
		ctx,
		HookInputPreSubmit,
		payload,
		dispatchConfig[InputPreSubmitPayload, InputPreSubmitPatch]{
			match:  matchInputPreSubmit,
			apply:  applyInputPreSubmitPatch,
			denied: inputPreSubmitPatchDenied,
			denyErr: func(_ InputPreSubmitPayload, report dispatchReport) error {
				return hookDeniedError(HookInputPreSubmit, report.DenyReason)
			},
		},
	)
}

// DispatchPromptPostAssemble runs the prompt.post_assemble hook pipeline.
func (h *Hooks) DispatchPromptPostAssemble(ctx context.Context, payload PromptPayload) (PromptPayload, error) {
	return h.executeDispatch(
		ctx,
		HookPromptPostAssemble,
		payload,
		dispatchConfig[PromptPayload, PromptPatch]{
			match:  matchPrompt,
			apply:  applyPromptPatch,
			denied: promptPatchDenied,
			denyErr: func(_ PromptPayload, report dispatchReport) error {
				return hookDeniedError(HookPromptPostAssemble, report.DenyReason)
			},
		},
	)
}

// DispatchEventPreRecord runs the event.pre_record hook dispatch.
func (h *Hooks) DispatchEventPreRecord(
	ctx context.Context,
	payload EventPreRecordPayload,
) (EventPreRecordPayload, error) {
	return h.executeDispatch(
		ctx,
		HookEventPreRecord,
		payload,
		dispatchConfig[EventPreRecordPayload, EventPreRecordPatch]{
			match: matchEventRecord,
			apply: applyNoop[EventPreRecordPayload, EventPreRecordPatch],
		},
	)
}

// DispatchEventPostRecord runs the event.post_record hook dispatch.
func (h *Hooks) DispatchEventPostRecord(
	ctx context.Context,
	payload EventPostRecordPayload,
) (EventPostRecordPayload, error) {
	return h.executeDispatch(
		ctx,
		HookEventPostRecord,
		payload,
		dispatchConfig[EventPostRecordPayload, EventPostRecordPatch]{
			match: matchEventRecord,
			apply: applyNoop[EventPostRecordPayload, EventPostRecordPatch],
		},
	)
}
