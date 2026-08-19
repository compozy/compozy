package cmdpalette

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

func (s *Service) Invoke(ctx context.Context, request InvokeRequest) (InvokeResult, error) {
	execution, resolved, err := s.prepareInvocation(ctx, request)
	if err != nil {
		return InvokeResult{}, err
	}
	guarded := resolved.Policy.SingleFlight
	if guarded && !s.acquireFlight(request.WorkspaceID, request.CommandID) {
		return InvokeResult{}, fmt.Errorf("%w: %s is already in flight", ErrAlreadyRunning, request.CommandID)
	}
	startedAt := s.now().UTC()
	result, err := s.executor.ExecuteAction(ctx, execution)
	if err != nil {
		if guarded {
			s.releaseFlight(request.WorkspaceID, request.CommandID)
		}
		s.emitInvocation(ctx, execution, "failed", "", startedAt)
		return InvokeResult{}, err
	}
	if result.ApprovalID != "" {
		return s.pendingApprovalResult(ctx, execution, resolved, result, startedAt)
	}
	if guarded {
		s.releaseFlight(request.WorkspaceID, request.CommandID)
	}
	s.recordDaemonUsage(ctx, execution)
	s.emitInvocation(ctx, execution, "ok", "", startedAt)
	return InvokeResult{
		Status: InvokeStatusOK, Result: append([]byte(nil), result.Result...), InvocationID: execution.InvocationID,
	}, nil
}

func (s *Service) prepareInvocation(
	ctx context.Context,
	request InvokeRequest,
) (ExecutionRequest, ResolvedCommand, error) {
	if ctx == nil {
		return ExecutionRequest{}, ResolvedCommand{}, errors.New("cmd palette: invoke context is required")
	}
	if request.WorkspaceID == "" {
		return ExecutionRequest{}, ResolvedCommand{}, errors.New("cmd palette: workspace ID is required")
	}
	if request.CommandID == "" {
		return ExecutionRequest{}, ResolvedCommand{}, fmt.Errorf("%w: empty command id", ErrCommandNotFound)
	}
	baseCatalog, err := s.Catalog(ctx, request.WorkspaceID, "")
	if err != nil {
		return ExecutionRequest{}, ResolvedCommand{}, err
	}
	command, exists := findCommand(baseCatalog.Commands, request.CommandID)
	if !exists {
		return ExecutionRequest{}, ResolvedCommand{}, fmt.Errorf(
			"%w: unknown command: %s", ErrCommandNotFound, request.CommandID,
		)
	}
	if fields := validateInvocationArguments(command.Arguments, request.Args); len(fields) > 0 {
		return ExecutionRequest{}, ResolvedCommand{}, &InvalidArgumentsError{Fields: fields}
	}
	clientID, err := s.resolveInvocationClient(ctx, request, command.Descriptor)
	if err != nil {
		return ExecutionRequest{}, ResolvedCommand{}, err
	}
	request.ClientID = clientID
	if err := s.authorizeInvocationClient(ctx, request); err != nil {
		return ExecutionRequest{}, ResolvedCommand{}, err
	}
	resolvedCatalog, err := s.Catalog(ctx, request.WorkspaceID, clientID)
	if err != nil {
		return ExecutionRequest{}, ResolvedCommand{}, err
	}
	resolved, exists := findCommand(resolvedCatalog.Commands, request.CommandID)
	if !exists {
		return ExecutionRequest{}, ResolvedCommand{}, fmt.Errorf(
			"%w: unknown command: %s", ErrCommandNotFound, request.CommandID,
		)
	}
	if !resolved.Available {
		return ExecutionRequest{}, ResolvedCommand{}, &UnavailableError{Reason: resolved.UnavailableReason}
	}
	invocationID := strings.TrimSpace(s.newID())
	if invocationID == "" {
		return ExecutionRequest{}, ResolvedCommand{}, errors.New("cmd palette: generated invocation ID is empty")
	}
	execution := ExecutionRequest{
		WorkspaceID:  request.WorkspaceID,
		InvocationID: invocationID,
		ClientID:     clientID,
		Descriptor:   cloneDescriptor(resolved.Descriptor),
		Args:         cloneAnyMap(request.Args),
	}
	if err := s.rejectDeferredSecrets(ctx, execution); err != nil {
		return ExecutionRequest{}, ResolvedCommand{}, err
	}
	return execution, resolved, nil
}

func (s *Service) pendingApprovalResult(
	ctx context.Context,
	execution ExecutionRequest,
	resolved ResolvedCommand,
	result ExecutionResult,
	startedAt time.Time,
) (InvokeResult, error) {
	if result.Completion == nil {
		if resolved.Policy.SingleFlight {
			s.releaseFlight(execution.WorkspaceID, execution.Descriptor.ID)
		}
		return InvokeResult{}, fmt.Errorf(
			"%w: approval-pending result requires completion fence", ErrInvalidExecution,
		)
	}
	if resolved.Policy.SingleFlight {
		go s.releaseFlightOnCompletion(execution.WorkspaceID, execution.Descriptor.ID, result.Completion)
	}
	if reader, ok := s.executor.(ApprovalCompletionReader); ok {
		go s.emitApprovalCompletion(
			context.WithoutCancel(ctx), execution, result.ApprovalID, startedAt, result.Completion, reader,
		)
	}
	return InvokeResult{
		Status: InvokeStatusApprovalPending, ApprovalID: result.ApprovalID, InvocationID: execution.InvocationID,
	}, nil
}

func (s *Service) emitApprovalCompletion(
	ctx context.Context,
	execution ExecutionRequest,
	approvalID string,
	startedAt time.Time,
	completion <-chan struct{},
	reader ApprovalCompletionReader,
) {
	<-completion
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	outcome, err := reader.ApprovalCompletionStatus(ctx, approvalID)
	if err != nil || outcome == "" {
		return
	}
	if outcome == "completed" {
		s.recordDaemonUsage(ctx, execution)
	}
	s.emitInvocation(ctx, execution, outcome, approvalID, startedAt)
}

func (s *Service) emitInvocation(
	ctx context.Context,
	execution ExecutionRequest,
	outcome string,
	approvalID string,
	startedAt time.Time,
) {
	now := s.now().UTC()
	s.emit(ctx, Event{
		Name: EventCommandInvoked, WorkspaceID: execution.WorkspaceID,
		CommandID: execution.Descriptor.ID, Source: execution.Descriptor.Source.ID(),
		ExecutionSite: execution.Descriptor.Action.Kind, Outcome: outcome,
		DurationMS:   max(0, now.Sub(startedAt).Milliseconds()),
		InvocationID: execution.InvocationID, ApprovalID: approvalID, OccurredAt: now,
	})
}

func (s *Service) resolveInvocationClient(
	ctx context.Context,
	request InvokeRequest,
	descriptor Descriptor,
) (ClientID, error) {
	needsClient := descriptorNeedsClient(descriptor)
	if request.ClientID != "" {
		if s.clients == nil {
			return "", ErrNoAttachedShell
		}
		clients, err := s.clients.Clients(ctx, request.WorkspaceID)
		if err != nil {
			return "", err
		}
		for _, client := range clients {
			if client.ID == request.ClientID {
				return request.ClientID, nil
			}
		}
		return "", ErrNoAttachedShell
	}
	if !needsClient {
		return "", nil
	}
	if s.clients == nil {
		return "", ErrNoAttachedShell
	}
	clients, err := s.clients.Clients(ctx, request.WorkspaceID)
	if err != nil {
		return "", err
	}
	sort.Slice(clients, func(left, right int) bool { return clients[left].ID < clients[right].ID })
	switch len(clients) {
	case 0:
		return "", ErrNoAttachedShell
	case 1:
		return clients[0].ID, nil
	default:
		ids := make([]ClientID, 0, len(clients))
		for _, client := range clients {
			ids = append(ids, client.ID)
		}
		return "", &MultipleClientsError{Clients: ids}
	}
}

func (s *Service) authorizeInvocationClient(ctx context.Context, request InvokeRequest) error {
	if request.ClientID == "" || request.Caller != CallerAttachedClient {
		return nil
	}
	if s.clients == nil || strings.TrimSpace(request.ClientToken) == "" {
		return ErrClientUnauthorized
	}
	if err := s.clients.Authorize(ctx, request.WorkspaceID, request.ClientID, request.ClientToken); err != nil {
		return fmt.Errorf("%w: %v", ErrClientUnauthorized, err)
	}
	return nil
}

func (s *Service) rejectDeferredSecrets(ctx context.Context, request ExecutionRequest) error {
	if !hasPasswordValue(request.Descriptor.Arguments, request.Args) {
		return nil
	}
	requiresApproval := request.Descriptor.Destructive && request.Descriptor.Action.Kind == ActionKindTool
	if preflight, ok := s.executor.(ApprovalPreflight); ok {
		resolved, err := preflight.ApprovalRequired(ctx, request)
		if err != nil {
			return fmt.Errorf("cmd palette: approval preflight: %w", err)
		}
		requiresApproval = resolved
	}
	if requiresApproval {
		return ErrCannotDeferSecrets
	}
	return nil
}

func (s *Service) releaseFlightOnCompletion(
	workspaceID WorkspaceID,
	commandID CommandID,
	completion <-chan struct{},
) {
	<-completion
	s.releaseFlight(workspaceID, commandID)
}

func descriptorNeedsClient(descriptor Descriptor) bool {
	if descriptor.Action.Kind != ActionKindTool {
		return true
	}
	for _, predicate := range descriptor.When {
		if clientContextKey(predicate.Key) {
			return true
		}
	}
	return false
}

func findCommand(commands []ResolvedCommand, id CommandID) (ResolvedCommand, bool) {
	index := sort.Search(len(commands), func(index int) bool { return commands[index].ID >= id })
	if index >= len(commands) || commands[index].ID != id {
		return ResolvedCommand{}, false
	}
	return commands[index], true
}

func validateInvocationArguments(arguments []Argument, supplied map[string]any) map[string]string {
	fields := make(map[string]string)
	declared := make(map[string]Argument, len(arguments))
	for _, argument := range arguments {
		declared[argument.Name] = argument
		value, exists := supplied[argument.Name]
		if argument.Required && (!exists || emptyArgumentValue(value)) {
			fields[argument.Name] = "required"
			continue
		}
		if exists && !emptyArgumentValue(value) && !argumentValueMatches(argument, value) {
			fields[argument.Name] = "invalid type"
		}
	}
	for name := range supplied {
		if _, exists := declared[name]; !exists {
			fields[name] = "unknown"
		}
	}
	return fields
}

func argumentValueMatches(argument Argument, value any) bool {
	switch argument.Type {
	case ArgumentTypeText, ArgumentTypePassword:
		_, ok := value.(string)
		return ok
	case ArgumentTypeCheckbox:
		_, ok := value.(bool)
		return ok
	case ArgumentTypeDropdown:
		selected, ok := value.(string)
		if !ok {
			return false
		}
		return slices.Contains(argument.Options, selected)
	default:
		return false
	}
}

func emptyArgumentValue(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func hasPasswordValue(arguments []Argument, supplied map[string]any) bool {
	for _, argument := range arguments {
		if argument.Type == ArgumentTypePassword && !emptyArgumentValue(supplied[argument.Name]) {
			return true
		}
	}
	return false
}
