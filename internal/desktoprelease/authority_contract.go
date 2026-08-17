package desktoprelease

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrChannelRefNotFound = errors.New("desktop release: channel ref not found")
var ErrCompareAndSwap = errors.New("desktop release: channel ref changed")

type ChannelCommit struct {
	SHA        string
	Generation Generation
	Files      map[string][]byte
}

type CommitRequest struct {
	ParentSHA  string
	Generation Generation
	Files      map[string][]byte
}

type AuthorityBackend interface {
	Repository() string
	ChannelRef(ctx context.Context, branch string) (string, error)
	FindOperation(ctx context.Context, branch, operationID string) (*ChannelCommit, error)
	Commit(ctx context.Context, request CommitRequest) (string, error)
	CompareAndSwapRef(ctx context.Context, branch, expectedSHA, nextSHA string) error
	ReleaseInventory(ctx context.Context, version string) ([]Artifact, error)
	UploadAsset(ctx context.Context, version, path string, artifact Artifact) error
	VerifyAsset(ctx context.Context, version string, artifact Artifact) error
	ReadCommit(ctx context.Context, sha string) (ChannelCommit, error)
	KnownGood(ctx context.Context, branch, version string) (ChannelCommit, error)
}

type PublishRequest struct {
	OperationID string
	Channel     string
	Version     string
	AssetDir    string
	ChannelDir  string
	PublishedAt time.Time
}

type RepairRequest struct {
	OperationID string
	Channel     string
	Version     string
	PublishedAt time.Time
}

type Authority struct {
	backend AuthorityBackend
}

func NewAuthority(backend AuthorityBackend) (*Authority, error) {
	if backend == nil {
		return nil, fmt.Errorf("desktop release: authority backend is required")
	}
	return &Authority{backend: backend}, nil
}
