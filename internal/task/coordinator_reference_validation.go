package task

func validateCoordinatorCompletionReferences(completion CoordinatorCompletion, path string) error {
	return ValidateReferenceSize(completion.RunID, nestedPath(path, "run_id"))
}

func validateCoordinatorPlanSnapshotReferences(plan CoordinatorCompletionPlan, path string) error {
	if err := ValidateReferenceSize(plan.Snapshot.LoopRunID, nestedPath(path, "snapshot.loop_run_id")); err != nil {
		return err
	}
	if plan.PostReserveSnapshot == nil {
		return nil
	}
	return ValidateReferenceSize(
		plan.PostReserveSnapshot.LoopRunID,
		nestedPath(path, "post_reserve_snapshot.loop_run_id"),
	)
}

func validateCoordinatorTaskSpecReferences(spec CoordinatorTaskSpec, path string) error {
	return ValidateReferenceSize(spec.TaskID, nestedPath(path, "task_id"))
}

func validateCoordinatorDependencySpecReferences(spec CoordinatorDependencySpec, path string) error {
	if err := ValidateReferenceSize(spec.TaskID, nestedPath(path, "task_id")); err != nil {
		return err
	}
	return ValidateReferenceSize(spec.DependsOnTaskID, nestedPath(path, "depends_on_task_id"))
}

func validateCoordinatorEnqueueSpecReferences(spec EnqueueSpec, path string) error {
	references := []struct {
		path  string
		value string
	}{
		{path: taskEvidenceIDKey, value: spec.TaskID},
		{path: runEvidenceIDKey, value: spec.RunID},
		{path: "loop_run_id", value: spec.LoopRunID},
		{path: "idempotency_key", value: spec.IdempotencyKey},
		{path: "designation_group_id", value: spec.DesignationGroupID},
	}
	for _, reference := range references {
		if err := ValidateReferenceSize(reference.value, nestedPath(path, reference.path)); err != nil {
			return err
		}
	}
	return nil
}

func validateCoordinatorStopSpecReferences(spec CoordinatorStopSpec, path string) error {
	if err := ValidateReferenceSize(spec.LoopRunID, nestedPath(path, "loop_run_id")); err != nil {
		return err
	}
	return ValidateReferenceSize(spec.ReasonCode, nestedPath(path, "reason_code"))
}

func validateCoordinatorWakeSpecReferences(spec CoordinatorWakeSpec, path string) error {
	if err := ValidateReferenceSize(spec.LoopRunID, nestedPath(path, "loop_run_id")); err != nil {
		return err
	}
	return ValidateReferenceSize(spec.IdempotencyKey, nestedPath(path, "idempotency_key"))
}

func validateCoordinatorTerminalReferences(terminal CoordinatorTerminal, path string) error {
	references := []struct {
		path  string
		value string
	}{
		{path: leaseStatusKey, value: terminal.Status},
		{path: "cause", value: terminal.Cause},
		{path: "reason_code", value: terminal.ReasonCode},
		{path: "gate_id", value: terminal.GateID},
	}
	for _, reference := range references {
		if err := ValidateReferenceSize(reference.value, nestedPath(path, reference.path)); err != nil {
			return err
		}
	}
	return nil
}
