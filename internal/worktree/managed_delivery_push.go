package worktree

import (
	"context"
	"strings"
)

// Always name the recorded destination and one branch. Operator upstream and
// push.default configuration cannot widen a managed delivery's side effects.
func (s *Service) pushManagedDelivery(ctx context.Context, j *managedDeliveryJournal) (ExitStepResult, error) {
	step := ExitStepResult{
		Phase:    ExitPhasePush,
		State:    exitStepCompleted,
		SHA:      j.Head,
		Upstream: "origin/" + j.Item.Branch,
	}
	ref := "refs/heads/" + j.Item.Branch
	urls, err := s.deliveryGitValue(ctx, j.Item, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return step, err
	}
	if len(j.RemoteURLs) != 1 || urls != j.RemoteURLs[0] {
		return step, refusal(
			ErrSafetyCheckFailed,
			"Managed delivery requires one identical fetch and push destination.",
		)
	}
	destination := j.RemoteURLs[0]
	remote, err := s.deliveryGitValue(ctx, j.Item, "ls-remote", "--heads", destination, ref)
	if err != nil {
		return step, err
	}
	if remote != "" {
		fields := strings.Fields(remote)
		if len(fields) != 2 || fields[1] != ref {
			return step, ErrSafetyCheckFailed
		}
		if fields[0] == j.Head {
			return step, nil
		}
	}
	if _, stderr, err := s.runner.Run(ctx, j.Item.Path, "push", destination, j.Head+":"+ref); err != nil {
		return step, refusal(ErrSafetyCheckFailed, exitCommandOutput(nil, stderr, err))
	}
	remote, err = s.deliveryGitValue(ctx, j.Item, "ls-remote", "--heads", destination, ref)
	if err != nil {
		return step, err
	}
	if remote != j.Head+"\t"+ref {
		return step, refusal(ErrSafetyCheckFailed, "Pushed branch does not match delivery HEAD.")
	}
	return step, nil
}
