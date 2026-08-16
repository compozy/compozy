package desktoprelease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func (a *Authority) Publish(ctx context.Context, request PublishRequest) (OperatorResult, error) {
	if err := validateOperationRequest(request.OperationID, request.Channel, request.Version); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if request.PublishedAt.IsZero() {
		return OperatorResult{}, errorWithCode(
			ErrorVerificationFailed,
			fmt.Errorf("desktop release: published_at is required"),
		)
	}
	branch := ChannelBranchPrefix + request.Channel
	if existing, err := a.backend.FindOperation(ctx, branch, request.OperationID); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	} else if existing != nil {
		verified, verifyErr := a.verifyRemoteInventory(ctx, existing.Generation.Version)
		if verifyErr != nil {
			return OperatorResult{}, verifyErr
		}
		return resultFromExisting(operationPublish, existing, verified), nil
	}
	before, previous, err := a.channelState(ctx, branch)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if previous.Version != "" {
		if err := AssertStrictlyGreater(request.Version, previous.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	compatibility, err := ReadCompatibility(filepath.Join(request.AssetDir, CompatibilityFile))
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorInventoryIncomplete, err)
	}
	if compatibility.RuntimeVersion != request.Version {
		return OperatorResult{}, errorWithCode(
			ErrorVerificationFailed,
			fmt.Errorf(
				"desktop release: compat runtime_version %s does not match release %s",
				compatibility.RuntimeVersion,
				request.Version,
			),
		)
	}
	if previous.Version != "" {
		if err := AssertCompatibleWithPrevious(compatibility.MinAppVersion, previous.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	verified, err := a.publishAndVerifyInventory(ctx, request.AssetDir, request.Version)
	if err != nil {
		return OperatorResult{}, err
	}
	generation := Generation{
		OperationID: request.OperationID, Operation: operationPublish, Version: request.Version,
		MinAppVersion: compatibility.MinAppVersion, PublishedAt: request.PublishedAt.UTC(),
	}
	files, err := prepareChannelFiles(request.ChannelDir, generation)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	auditCommit, err := a.backend.Commit(ctx, CommitRequest{ParentSHA: before, Generation: generation, Files: files})
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if err := a.backend.CompareAndSwapRef(ctx, branch, before, auditCommit); err != nil {
		if errors.Is(err, ErrCompareAndSwap) {
			return OperatorResult{}, errorWithCode(ErrorChannelCASConflict, err)
		}
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	return OperatorResult{
		Operation: operationPublish, ChannelRefBefore: before, ChannelRefAfter: auditCommit,
		VerifiedInventory: verified, AuditCommit: auditCommit, Outcome: "published",
	}, nil
}

func (a *Authority) Repair(ctx context.Context, request RepairRequest) (OperatorResult, error) {
	if err := validateOperationRequest(request.OperationID, request.Channel, request.Version); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if request.PublishedAt.IsZero() {
		return OperatorResult{}, errorWithCode(
			ErrorVerificationFailed,
			fmt.Errorf("desktop release: published_at is required"),
		)
	}
	branch := ChannelBranchPrefix + request.Channel
	if existing, err := a.backend.FindOperation(ctx, branch, request.OperationID); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	} else if existing != nil {
		verified, verifyErr := a.verifyRemoteInventory(ctx, existing.Generation.Version)
		if verifyErr != nil {
			return OperatorResult{}, verifyErr
		}
		return resultFromExisting(operationRepair, existing, verified), nil
	}
	before, _, err := a.channelState(ctx, branch)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	knownGood, err := a.backend.KnownGood(ctx, branch, request.Version)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	verified, err := a.verifyRemoteInventory(ctx, knownGood.Generation.Version)
	if err != nil {
		return OperatorResult{}, err
	}
	generation := knownGood.Generation
	generation.OperationID = request.OperationID
	generation.Operation = operationRepair
	generation.PublishedAt = request.PublishedAt.UTC()
	generation.SourceCommit = knownGood.SHA
	files := cloneFiles(knownGood.Files)
	generationBytes, err := canonicalJSON(generation)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	files[filepath.Join(ChannelDirectory, GenerationFile)] = generationBytes
	auditCommit, err := a.backend.Commit(ctx, CommitRequest{ParentSHA: before, Generation: generation, Files: files})
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if err := a.backend.CompareAndSwapRef(ctx, branch, before, auditCommit); err != nil {
		if errors.Is(err, ErrCompareAndSwap) {
			return OperatorResult{}, errorWithCode(ErrorChannelCASConflict, err)
		}
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	return OperatorResult{
		Operation: operationRepair, ChannelRefBefore: before, ChannelRefAfter: auditCommit,
		VerifiedInventory: verified, AuditCommit: auditCommit, Outcome: "repaired",
	}, nil
}

func (a *Authority) channelState(ctx context.Context, branch string) (string, Generation, error) {
	sha, err := a.backend.ChannelRef(ctx, branch)
	if errors.Is(err, ErrChannelRefNotFound) {
		return "", Generation{}, nil
	}
	if err != nil {
		return "", Generation{}, err
	}
	commit, err := a.backend.ReadCommit(ctx, sha)
	if err != nil {
		return "", Generation{}, err
	}
	return sha, commit.Generation, nil
}

func (a *Authority) publishAndVerifyInventory(
	ctx context.Context,
	dir, version string,
) ([]Artifact, error) {
	local, err := InspectDesktopInventory(ctx, dir, version)
	if err != nil {
		return nil, errorWithCode(ErrorInventoryIncomplete, err)
	}
	compatibilityArtifact, err := inspectArtifact(filepath.Join(dir, CompatibilityFile))
	if err != nil {
		return nil, errorWithCode(ErrorInventoryIncomplete, err)
	}
	local = append(local, compatibilityArtifact)
	if err := VerifyChecksumsCatalog(ctx, filepath.Join(dir, ChecksumsFile), local); err != nil {
		return nil, errorWithCode(ErrorVerificationFailed, err)
	}
	for _, artifact := range local {
		if err := a.backend.UploadAsset(ctx, version, filepath.Join(dir, artifact.Name), artifact); err != nil {
			return nil, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	return a.verifyArtifacts(ctx, version, local)
}

func (a *Authority) verifyRemoteInventory(ctx context.Context, version string) ([]Artifact, error) {
	remote, err := a.backend.ReleaseInventory(ctx, version)
	if err != nil {
		return nil, errorWithCode(ErrorVerificationFailed, err)
	}
	required := append(DesktopArtifactNames(version), CompatibilityFile)
	byName := make(map[string]Artifact, len(remote))
	for _, artifact := range remote {
		if artifact.Size <= 0 {
			return nil, errorWithCode(
				ErrorInventoryIncomplete,
				fmt.Errorf("desktop release: release %s has empty artifact %s", version, artifact.Name),
			)
		}
		byName[artifact.Name] = artifact
	}
	artifacts := make([]Artifact, 0, len(required))
	for _, name := range required {
		artifact, ok := byName[name]
		if !ok {
			return nil, errorWithCode(
				ErrorInventoryIncomplete,
				fmt.Errorf("desktop release: release %s is missing %s", version, name),
			)
		}
		artifacts = append(artifacts, artifact)
	}
	return a.verifyArtifacts(ctx, version, artifacts)
}

func (a *Authority) verifyArtifacts(ctx context.Context, version string, artifacts []Artifact) ([]Artifact, error) {
	for _, artifact := range artifacts {
		if err := a.backend.VerifyAsset(ctx, version, artifact); err != nil {
			return nil, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts, nil
}

func prepareChannelFiles(dir string, generation Generation) (map[string][]byte, error) {
	contents, err := canonicalJSON(generation)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, GenerationFile), contents, 0o600); err != nil {
		return nil, fmt.Errorf("desktop release: write generation file: %w", err)
	}
	files, err := LoadChannelFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{ManifestMac, ManifestLinux} {
		manifest, decodeErr := decodeUpdateManifest(files[filepath.Join(ChannelDirectory, name)], name)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if manifest.Version != generation.Version {
			return nil, fmt.Errorf(
				"desktop release: manifest %s version %s does not match generation %s",
				name,
				manifest.Version,
				generation.Version,
			)
		}
		if inventoryErr := validateManifestInventory(manifest, name, generation.Version); inventoryErr != nil {
			return nil, inventoryErr
		}
	}
	return files, nil
}

func validateOperationRequest(operationID, channel, version string) error {
	if operationID == "" || len(operationID) > 128 {
		return fmt.Errorf("desktop release: operation id must contain 1 to 128 safe characters")
	}
	for _, character := range operationID {
		if !isSafeOperationIDCharacter(character) {
			return fmt.Errorf("desktop release: operation id contains unsupported character %q", character)
		}
	}
	if err := ValidateDesktopChannel(channel); err != nil {
		return err
	}
	return ValidateVersion(version)
}

func isSafeOperationIDCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '-' || character == '_' || character == '.'
}

func resultFromExisting(operation string, commit *ChannelCommit, verified []Artifact) OperatorResult {
	return OperatorResult{
		Operation: operation, ChannelRefBefore: commit.SHA, ChannelRefAfter: commit.SHA,
		VerifiedInventory: verified, AuditCommit: commit.SHA, Outcome: "already_completed",
	}
}

func cloneFiles(source map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(source))
	for name, contents := range source {
		cloned[name] = append([]byte(nil), contents...)
	}
	return cloned
}

func DecodeGeneration(contents []byte) (Generation, error) {
	var generation Generation
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&generation); err != nil {
		return Generation{}, fmt.Errorf("desktop release: decode generation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Generation{}, fmt.Errorf("desktop release: generation contains multiple JSON documents")
		}
		return Generation{}, fmt.Errorf("desktop release: decode generation trailing data: %w", err)
	}
	if generation.OperationID == "" || generation.Operation == "" {
		return Generation{}, fmt.Errorf("desktop release: generation audit identity is incomplete")
	}
	if generation.Operation != operationPublish && generation.Operation != operationRepair {
		return Generation{}, fmt.Errorf("desktop release: unsupported generation operation %q", generation.Operation)
	}
	if err := ValidateVersion(generation.Version); err != nil {
		return Generation{}, err
	}
	if err := ValidateVersion(generation.MinAppVersion); err != nil {
		return Generation{}, fmt.Errorf("desktop release: generation min_app_version: %w", err)
	}
	if generation.PublishedAt.IsZero() {
		return Generation{}, fmt.Errorf("desktop release: generation published_at is required")
	}
	if generation.Operation == operationRepair && generation.SourceCommit == "" {
		return Generation{}, fmt.Errorf("desktop release: repair generation source_commit is required")
	}
	return generation, nil
}
