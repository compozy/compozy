package cmdpalette

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ViewToolSource struct {
	Tool     string `json:"tool"`
	ReadOnly bool   `json:"read_only"`
}

type ViewDescriptor struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Kind      ViewKind        `json:"kind"`
	Source    *ViewToolSource `json:"source,omitempty"`
	Program   bool            `json:"program,omitempty"`
	Extension string          `json:"extension,omitempty"`
}

type ViewSnapshot struct {
	Descriptor  ViewDescriptor
	Payload     ViewPayload
	Revision    string
	StreamEpoch string
}

type ViewPatchEvent struct {
	Sequence    int64
	StreamEpoch string
	Patch       ViewPatch
}

type ViewSourceProvider interface {
	OpenSource(context.Context, WorkspaceID, string) (ViewPayload, error)
}

type ViewPatchSubscribeRequest struct {
	Workspace   WorkspaceID
	ViewID      string
	After       int64
	StreamEpoch string
}

type ViewPatchSubscriber interface {
	SubscribeViewPatches(
		context.Context,
		ViewPatchSubscribeRequest,
	) (<-chan ViewPatchEvent, func(), error)
}

type ViewProviderRegistration struct {
	Descriptor ViewDescriptor
	Provider   ViewSourceProvider
}

// DynamicViewProvider resolves workspace-scoped extension view descriptors.
type DynamicViewProvider interface {
	ProvideViews(context.Context, WorkspaceID) ([]ViewDescriptor, error)
	ViewSourceProvider
}

// ViewSourceService is the Tier-1 read surface consumed by both API transports.
type ViewSourceService interface {
	ResolveView(context.Context, WorkspaceID, string) (ViewDescriptor, error)
	OpenSource(context.Context, WorkspaceID, string) (ViewSnapshot, error)
	SubscribeViewPatches(
		context.Context,
		ViewPatchSubscribeRequest,
	) (ViewSnapshot, <-chan ViewPatchEvent, func(), error)
}

// ViewSessionService is the Tier-2 programmable-view session authority.
type ViewSessionService interface {
	OpenSession(context.Context, ViewSessionOpenRequest) (ViewSessionOpenResult, error)
	AdmitEvent(context.Context, SessionToken, ViewEvent) error
	PublishFrame(context.Context, SessionToken, ViewFrame) error
	AckEffects(context.Context, SessionToken, []string) error
	SubscribeSessionFrames(
		context.Context,
		SessionToken,
	) (ViewFrame, <-chan ViewFrame, func(), error)
	CloseSession(context.Context, SessionToken, string) error
	CloseClientSessions(context.Context, WorkspaceID, ClientID) error
	InvalidateInstance(context.Context, WorkspaceID, string, uint64) error
}

// ViewService owns both declarative views and programmable sessions.
type ViewService interface {
	ViewSourceService
	ViewSessionService
}

// WithViewProgramProvider registers the single view.provider runtime.
func WithViewProgramProvider(provider ViewProgramProvider) Option {
	return func(service *Service) error {
		if provider == nil {
			return errors.New("cmd palette view: program provider is required")
		}
		service.viewPrograms = provider
		return nil
	}
}

// WithViewProviders registers static declarative and programmable view descriptors.
func WithViewProviders(registrations []ViewProviderRegistration) Option {
	return func(service *Service) error {
		if err := validateViewProviderRegistrations(registrations); err != nil {
			return err
		}
		service.viewProviders = append([]ViewProviderRegistration(nil), registrations...)
		return nil
	}
}

// WithDynamicViewProvider registers one workspace-scoped view projection.
func WithDynamicViewProvider(provider DynamicViewProvider) Option {
	return func(service *Service) error {
		if provider == nil {
			return errors.New("cmd palette view: dynamic provider is required")
		}
		service.dynamicViews = append(service.dynamicViews, provider)
		return nil
	}
}

func (s *Service) ResolveView(
	ctx context.Context,
	workspaceID WorkspaceID,
	viewID string,
) (ViewDescriptor, error) {
	descriptor, _, err := s.lookupView(ctx, workspaceID, viewID)
	return descriptor, err
}

func (s *Service) OpenSource(
	ctx context.Context,
	workspaceID WorkspaceID,
	viewID string,
) (ViewSnapshot, error) {
	descriptor, provider, err := s.resolveViewProvider(ctx, workspaceID, viewID)
	if err != nil {
		return ViewSnapshot{}, err
	}
	if err := requireDeclarativeViewSource(descriptor); err != nil {
		return ViewSnapshot{}, err
	}
	payload, err := provider.OpenSource(ctx, workspaceID, descriptor.ID)
	if err != nil {
		return ViewSnapshot{}, fmt.Errorf("cmd palette view: open %q: %w", descriptor.ID, err)
	}
	validated, err := ValidateViewPayload(descriptor.Kind, payload, nil, nil)
	if err != nil {
		return ViewSnapshot{}, err
	}
	revision, err := viewPayloadRevision(validated)
	if err != nil {
		return ViewSnapshot{}, err
	}
	return ViewSnapshot{
		Descriptor: cloneViewDescriptor(descriptor), Payload: validated,
		Revision: revision, StreamEpoch: s.viewStreamEpoch,
	}, nil
}

func (s *Service) SubscribeViewPatches(
	ctx context.Context,
	request ViewPatchSubscribeRequest,
) (ViewSnapshot, <-chan ViewPatchEvent, func(), error) {
	request.ViewID = strings.TrimSpace(request.ViewID)
	request.StreamEpoch = strings.TrimSpace(request.StreamEpoch)
	if err := validateViewPatchSubscribeRequest(request); err != nil {
		return ViewSnapshot{}, nil, nil, err
	}
	if request.StreamEpoch == "" {
		request.StreamEpoch = s.viewStreamEpoch
	}
	descriptor, provider, err := s.resolveViewProvider(ctx, request.Workspace, request.ViewID)
	if err != nil {
		return ViewSnapshot{}, nil, nil, err
	}
	if err := requireDeclarativeViewSource(descriptor); err != nil {
		return ViewSnapshot{}, nil, nil, err
	}
	subscriber, ok := provider.(ViewPatchSubscriber)
	if !ok {
		return ViewSnapshot{}, nil, nil, ErrViewPatchStreamUnavailable
	}
	events, cancel, err := subscriber.SubscribeViewPatches(ctx, request)
	if err != nil {
		return ViewSnapshot{}, nil, nil, err
	}
	snapshot, err := s.OpenSource(ctx, request.Workspace, request.ViewID)
	if err != nil {
		cancel()
		return ViewSnapshot{}, nil, nil, err
	}
	return snapshot, events, cancel, nil
}

func validateViewPatchSubscribeRequest(request ViewPatchSubscribeRequest) error {
	if request.After < 0 {
		return ErrViewInvalidSequence
	}
	if request.After > 0 && strings.TrimSpace(request.StreamEpoch) == "" {
		return ErrViewStreamEpochRequired
	}
	return nil
}

func requireDeclarativeViewSource(descriptor ViewDescriptor) error {
	if descriptor.Program || descriptor.Source == nil {
		return viewValidationError("source", "view %q is not declarative", descriptor.ID)
	}
	if !descriptor.Source.ReadOnly {
		return viewValidationError("source.tool", "view source must be read-only")
	}
	return nil
}

func (s *Service) resolveViewProvider(
	ctx context.Context,
	workspaceID WorkspaceID,
	viewID string,
) (ViewDescriptor, ViewSourceProvider, error) {
	return s.lookupView(ctx, workspaceID, viewID)
}

func (s *Service) lookupView(
	ctx context.Context,
	workspaceID WorkspaceID,
	viewID string,
) (ViewDescriptor, ViewSourceProvider, error) {
	if ctx == nil {
		return ViewDescriptor{}, nil, errors.New("cmd palette view: context is required")
	}
	if workspaceID == "" {
		return ViewDescriptor{}, nil, errors.New("cmd palette view: workspace ID is required")
	}
	viewID = strings.TrimSpace(viewID)
	for _, registration := range s.viewProviders {
		if registration.Descriptor.ID == viewID {
			return cloneViewDescriptor(registration.Descriptor), registration.Provider, nil
		}
	}
	for _, provider := range s.dynamicViews {
		descriptors, err := provider.ProvideViews(ctx, workspaceID)
		if err != nil {
			return ViewDescriptor{}, nil, fmt.Errorf("cmd palette view: project dynamic views: %w", err)
		}
		for _, descriptor := range descriptors {
			if descriptor.ID == viewID {
				return cloneViewDescriptor(descriptor), provider, nil
			}
		}
	}
	return ViewDescriptor{}, nil, &ViewNotFoundError{ViewID: viewID}
}

func validateViewProviderRegistrations(registrations []ViewProviderRegistration) error {
	seen := make(map[string]struct{}, len(registrations))
	for index, registration := range registrations {
		path := fmt.Sprintf("views[%d]", index)
		descriptor := registration.Descriptor
		if registration.Provider == nil {
			return viewValidationError(path+".source", "provider is required")
		}
		if strings.TrimSpace(descriptor.ID) == "" || !commandIDPattern.MatchString(descriptor.ID) {
			return viewValidationError(path+".id", "must be a lowercase dotted identifier")
		}
		if strings.TrimSpace(descriptor.Title) == "" {
			return viewValidationError(path+".title", "is required")
		}
		switch descriptor.Kind {
		case ViewKindList, ViewKindDetail, ViewKindForm, ViewKindGrid:
		default:
			return viewValidationError(path+".kind", "unknown view kind %q", descriptor.Kind)
		}
		if descriptor.Program == (descriptor.Source != nil) {
			return viewValidationError(path, "exactly one of program or source is required")
		}
		if descriptor.Source != nil && strings.TrimSpace(descriptor.Source.Tool) == "" {
			return viewValidationError(path+".source.tool", "is required")
		}
		if _, exists := seen[descriptor.ID]; exists {
			return viewValidationError(path+".id", "duplicate %q", descriptor.ID)
		}
		seen[descriptor.ID] = struct{}{}
	}
	return nil
}

func viewPayloadRevision(payload ViewPayload) (string, error) {
	wire, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cmd palette view: encode revision payload: %w", err)
	}
	digest := sha256.Sum256(wire)
	return "vr_" + hex.EncodeToString(digest[:]), nil
}

func cloneViewDescriptor(descriptor ViewDescriptor) ViewDescriptor {
	cloned := descriptor
	if descriptor.Source != nil {
		source := *descriptor.Source
		cloned.Source = &source
	}
	return cloned
}

// UnknownViewKindPayload is the renderer-safe Null Object for a runtime kind gap.
func UnknownViewKindPayload(kind ViewKind) ViewPayload {
	return ViewPayload{
		View: ViewContractVersion,
		Empty: &EmptyState{
			Title: "View unavailable",
			Hint:  fmt.Sprintf("This host cannot render the %q view kind.", kind),
		},
	}
}

var (
	_ ViewSourceService  = (*Service)(nil)
	_ ViewSessionService = (*Service)(nil)
	_ ViewService        = (*Service)(nil)
)
