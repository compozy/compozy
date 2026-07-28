package daytona

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	daytonasdk "github.com/daytonaio/daytona/libs/sdk-go/pkg/daytona"
	daytonaerrors "github.com/daytonaio/daytona/libs/sdk-go/pkg/errors"
	daytonaoptions "github.com/daytonaio/daytona/libs/sdk-go/pkg/options"
	daytonatypes "github.com/daytonaio/daytona/libs/sdk-go/pkg/types"
)

var errSandboxNotFound = stderrors.New("sandbox/daytona: sandbox not found")

type sandboxClientFactory func(config clientConfig) (sandboxClient, error)

type clientConfig struct {
	APIURL string
	Target string
}

type createSandboxRequest struct {
	Name               string
	Labels             map[string]string
	EnvVars            map[string]string
	Public             bool
	Snapshot           string
	Image              string
	AutoStopMinutes    *int
	AutoArchiveMinutes *int
	Timeout            time.Duration
}

type sandboxClient interface {
	Create(ctx context.Context, req createSandboxRequest) (daytonaSandbox, error)
	Get(ctx context.Context, id string) (daytonaSandbox, error)
	FindOne(ctx context.Context, labels map[string]string) (daytonaSandbox, error)
}

type daytonaSandbox interface {
	ID() string
	Name() string
	Start(ctx context.Context) error
	Archive(ctx context.Context) error
	Delete(ctx context.Context) error
	WorkingDir(ctx context.Context) (string, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, content []byte) error
}

func newSDKClient(config clientConfig) (sandboxClient, error) {
	client, err := daytonasdk.NewClientWithConfig(&daytonatypes.DaytonaConfig{
		APIUrl: normalizeAPIURL(config.APIURL),
		Target: config.Target,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: create Daytona SDK client: %w", err)
	}
	return &sdkClient{client: client}, nil
}

type sdkClient struct {
	client *daytonasdk.Client
}

func (c *sdkClient) Create(ctx context.Context, req createSandboxRequest) (daytonaSandbox, error) {
	base := daytonatypes.SandboxBaseParams{
		Name:     req.Name,
		EnvVars:  req.EnvVars,
		Labels:   req.Labels,
		Public:   req.Public,
		Language: daytonatypes.CodeLanguagePython,
	}
	if req.AutoStopMinutes != nil {
		base.AutoStopInterval = req.AutoStopMinutes
	}
	if req.AutoArchiveMinutes != nil {
		base.AutoArchiveInterval = req.AutoArchiveMinutes
	}

	var params any
	switch {
	case req.Snapshot != "":
		params = daytonatypes.SnapshotParams{
			SandboxBaseParams: base,
			Snapshot:          req.Snapshot,
		}
	case req.Image != "":
		params = daytonatypes.ImageParams{
			SandboxBaseParams: base,
			Image:             req.Image,
		}
	default:
		params = daytonatypes.SnapshotParams{SandboxBaseParams: base}
	}

	opts := []func(*daytonaoptions.CreateSandbox){}
	if req.Timeout > 0 {
		opts = append(opts, daytonaoptions.WithTimeout(req.Timeout))
	}

	sandbox, err := c.client.Create(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: create sandbox: %w", err)
	}
	return sdkSandbox{sandbox: sandbox}, nil
}

func (c *sdkClient) Get(ctx context.Context, id string) (daytonaSandbox, error) {
	sandbox, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, mapSDKNotFound("get sandbox", err)
	}
	return sdkSandbox{sandbox: sandbox}, nil
}

func (c *sdkClient) FindOne(ctx context.Context, labels map[string]string) (daytonaSandbox, error) {
	limit := 1
	iter := c.client.List(ctx, &daytonasdk.ListSandboxesQuery{
		Labels: labels,
		Limit:  &limit,
	})
	if !iter.Next() {
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("sandbox/daytona: list sandboxes by labels: %w", err)
		}
		return nil, errSandboxNotFound
	}
	return sdkSandbox{sandbox: iter.Value()}, nil
}

type sdkSandbox struct {
	sandbox *daytonasdk.Sandbox
}

func (s sdkSandbox) ID() string {
	if s.sandbox == nil {
		return ""
	}
	return s.sandbox.ID
}

func (s sdkSandbox) Name() string {
	if s.sandbox == nil {
		return ""
	}
	return s.sandbox.Name
}

func (s sdkSandbox) Start(ctx context.Context) error {
	if s.sandbox == nil {
		return errSandboxNotFound
	}
	if err := s.sandbox.Start(ctx); err != nil {
		return fmt.Errorf("sandbox/daytona: start sandbox %q: %w", s.sandbox.ID, err)
	}
	return nil
}

func (s sdkSandbox) Archive(ctx context.Context) error {
	if s.sandbox == nil {
		return errSandboxNotFound
	}
	if err := s.sandbox.Archive(ctx); err != nil {
		return fmt.Errorf("sandbox/daytona: archive sandbox %q: %w", s.sandbox.ID, err)
	}
	return nil
}

func (s sdkSandbox) Delete(ctx context.Context) error {
	if s.sandbox == nil {
		return errSandboxNotFound
	}
	if err := s.sandbox.Delete(ctx); err != nil {
		return fmt.Errorf("sandbox/daytona: delete sandbox %q: %w", s.sandbox.ID, err)
	}
	return nil
}

func (s sdkSandbox) WorkingDir(ctx context.Context) (string, error) {
	if s.sandbox == nil {
		return "", errSandboxNotFound
	}
	dir, err := s.sandbox.GetWorkingDir(ctx)
	if err != nil {
		return "", fmt.Errorf("sandbox/daytona: get sandbox %q working dir: %w", s.sandbox.ID, err)
	}
	return dir, nil
}

func (s sdkSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if s.sandbox == nil {
		return nil, errSandboxNotFound
	}
	content, err := s.sandbox.FileSystem.DownloadFile(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: read file %q: %w", path, err)
	}
	return content, nil
}

func (s sdkSandbox) WriteFile(ctx context.Context, path string, content []byte) error {
	if s.sandbox == nil {
		return errSandboxNotFound
	}
	if err := s.sandbox.FileSystem.UploadFile(ctx, content, path); err != nil {
		return fmt.Errorf("sandbox/daytona: write file %q: %w", path, err)
	}
	return nil
}

func mapSDKNotFound(operation string, err error) error {
	if notFound, ok := stderrors.AsType[*daytonaerrors.DaytonaNotFoundError](err); ok && notFound != nil {
		return fmt.Errorf("%w: %s", errSandboxNotFound, operation)
	}
	return fmt.Errorf("sandbox/daytona: %s: %w", operation, err)
}
