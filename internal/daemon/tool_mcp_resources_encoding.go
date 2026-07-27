package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/resources"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func validateAndEncodeTool(
	ctx context.Context,
	codec resources.KindCodec[toolspkg.Tool],
	scope resources.ResourceScope,
	spec toolspkg.Tool,
) (toolspkg.Tool, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return toolspkg.Tool{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return toolspkg.Tool{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return toolspkg.Tool{}, nil, err
	}
	return validated, canonical, nil
}

func validateAndEncodeMCPServer(
	ctx context.Context,
	codec resources.KindCodec[compozyconfig.MCPServer],
	scope resources.ResourceScope,
	spec compozyconfig.MCPServer,
) (compozyconfig.MCPServer, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return compozyconfig.MCPServer{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return compozyconfig.MCPServer{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return compozyconfig.MCPServer{}, nil, err
	}
	return validated, canonical, nil
}

func managedResourceID(
	prefix string,
	scope resources.ResourceScope,
	sourceKey string,
	encoded []byte,
) string {
	sum := sha256.Sum256([]byte(
		string(scope.Kind) + "\x00" + scope.ID + "\x00" + strings.TrimSpace(sourceKey) + "\x00" + string(encoded),
	))
	return prefix + hex.EncodeToString(sum[:12])
}

func cloneResourceRecords[T any](records []resources.Record[T], cloneSpec func(T) T) []resources.Record[T] {
	if len(records) == 0 {
		return nil
	}
	cloned := make([]resources.Record[T], len(records))
	for idx := range records {
		cloned[idx] = cloneResourceRecord(records[idx], cloneSpec)
	}
	return cloned
}

func cloneResourceRecord[T any](record resources.Record[T], cloneSpec func(T) T) resources.Record[T] {
	if cloneSpec != nil {
		record.Spec = cloneSpec(record.Spec)
	}
	return record
}

func cloneDaemonMCPServer(src compozyconfig.MCPServer) compozyconfig.MCPServer {
	return compozyconfig.MCPServer{
		Name:      src.Name,
		Transport: src.Transport,
		Command:   src.Command,
		Args:      slices.Clone(src.Args),
		Env:       cloneStringMap(src.Env),
		SecretEnv: cloneStringMap(src.SecretEnv),
		URL:       src.URL,
		Auth: compozyconfig.MCPAuthConfig{
			Type:             src.Auth.Type,
			IssuerURL:        src.Auth.IssuerURL,
			MetadataURL:      src.Auth.MetadataURL,
			AuthorizationURL: src.Auth.AuthorizationURL,
			TokenURL:         src.Auth.TokenURL,
			RevocationURL:    src.Auth.RevocationURL,
			ClientID:         src.Auth.ClientID,
			ClientSecretRef:  src.Auth.ClientSecretRef,
			Scopes:           slices.Clone(src.Auth.Scopes),
		},
	}
}
