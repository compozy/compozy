package hooks

func cloneContextBlocks(blocks []ContextBlock) []ContextBlock {
	if blocks == nil {
		return nil
	}

	cloned := make([]ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		cloned = append(cloned, ContextBlock{
			Kind:     block.Kind,
			Text:     block.Text,
			Metadata: cloneStringMap(block.Metadata),
		})
	}
	return cloned
}

func cloneRawMessage(payload []byte) []byte {
	if payload == nil {
		return nil
	}

	return append([]byte(nil), payload...)
}

func sessionCreatePatchDenied(patch SessionCreatePatch) bool {
	return patch.Deny
}

func sandboxPreparePatchDenied(patch SandboxPreparePatch) bool {
	return patch.Deny
}

func sandboxSyncBeforePatchDenied(patch SandboxSyncBeforePatch) bool {
	return patch.Deny
}

func sandboxStopPatchDenied(patch SandboxStopPatch) bool {
	return patch.Deny
}

func inputPreSubmitPatchDenied(patch InputPreSubmitPatch) bool {
	return patch.Deny
}

func promptPatchDenied(patch PromptPatch) bool {
	return patch.Deny
}

func agentStartPatchDenied(patch AgentStartPatch) bool {
	return patch.Deny
}

func turnPatchDenied(patch TurnPatch) bool {
	return patch.Deny
}

func messagePatchDenied(patch MessagePatch) bool {
	return patch.Deny
}

func toolCallPatchDenied(patch ToolCallPatch) bool {
	return patch.Deny
}

func toolResultPatchDenied(patch ToolResultPatch) bool {
	return patch.Deny
}

func contextCompactionPatchDenied(patch ContextCompactionPatch) bool {
	return patch.Deny
}

func coordinatorSpawnPatchDenied(patch CoordinatorSpawnPatch) bool {
	return patch.Deny
}

func taskRunPreClaimPatchDenied(patch TaskRunPreClaimPatch) bool {
	return patch.Deny
}

func loopControlPatchDenied(patch LoopControlPatch) bool {
	return patch.Deny
}

func spawnCreatePatchDenied(patch SpawnCreatePatch) bool {
	return patch.Deny
}
