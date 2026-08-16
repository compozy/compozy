package extensionpkg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ toolspkg.ProjectionGenerationProvider = (*ExtensionToolProvider)(nil)

// ProjectionGeneration reports manifest and live runtime state used by tool projection.
func (p *ExtensionToolProvider) ProjectionGeneration(
	ctx context.Context,
	scope toolspkg.Scope,
) (string, bool) {
	if err := extensionProviderContextErr(ctx); err != nil {
		return "", false
	}
	manifestTools, manifestFingerprint, err := p.manifestToolsWithFingerprint(ctx, scope)
	if err != nil {
		return "", false
	}
	runtimeFingerprint, known := p.runtimeProjectionFingerprint(ctx, manifestTools)
	if !known {
		return "", false
	}
	return extensionProjectionGenerationToken(manifestFingerprint, runtimeFingerprint), true
}

func (p *ExtensionToolProvider) runtimeProjectionFingerprint(
	ctx context.Context,
	manifestTools []extensionManifestTool,
) (string, bool) {
	var fingerprint strings.Builder
	runtime := p.runtimeInstance()
	if runtime == nil {
		fingerprint.WriteString("runtime:none")
		return fingerprint.String(), true
	}
	seen := make(map[InstanceKey]struct{})
	for i := range manifestTools {
		key := manifestTools[i].key.Normalize()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		var snapshot *Extension
		var err error
		if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
			snapshot, err = scoped.GetForInstance(key)
		} else {
			snapshot, err = runtime.Get(key.Name)
		}
		if err != nil {
			return "", false
		}
		appendExtensionProjectionFingerprint(&fingerprint, key, snapshot)
		descriptors, err := extensionProjectionRuntimeDescriptors(ctx, runtime, key)
		if err != nil {
			return "", false
		}
		canonicalDescriptors, err := canonicalProjectionRuntimeDescriptors(descriptors)
		if err != nil {
			return "", false
		}
		encoded, err := json.Marshal(canonicalDescriptors)
		if err != nil {
			return "", false
		}
		fingerprint.Write(encoded)
		fingerprint.WriteByte(0)
	}
	return fingerprint.String(), true
}

func canonicalProjectionRuntimeDescriptors(
	src []toolspkg.ExtensionToolRuntimeDescriptor,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	descriptors := cloneRuntimeToolDescriptors(src)
	keyed := make([]canonicalProjectionRuntimeDescriptor, len(descriptors))
	for i := range descriptors {
		descriptors[i].Capabilities = canonicalProjectionCapabilities(descriptors[i].Capabilities)
		inputSchema, err := canonicalProjectionSchema(descriptors[i].InputSchema)
		if err != nil {
			return nil, err
		}
		descriptors[i].InputSchema = inputSchema
		outputSchema, err := canonicalProjectionSchema(descriptors[i].OutputSchema)
		if err != nil {
			return nil, err
		}
		descriptors[i].OutputSchema = outputSchema
		key, err := json.Marshal(descriptors[i])
		if err != nil {
			return nil, err
		}
		keyed[i] = canonicalProjectionRuntimeDescriptor{
			descriptor: descriptors[i],
			key:        key,
		}
	}
	slices.SortFunc(keyed, func(left, right canonicalProjectionRuntimeDescriptor) int {
		return bytes.Compare(left.key, right.key)
	})
	for i := range keyed {
		descriptors[i] = keyed[i].descriptor
	}
	return descriptors, nil
}

type canonicalProjectionRuntimeDescriptor struct {
	descriptor toolspkg.ExtensionToolRuntimeDescriptor
	key        []byte
}

func canonicalProjectionCapabilities(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	canonical := slices.Clone(values)
	slices.Sort(canonical)
	return slices.Compact(canonical)
}

func canonicalProjectionSchema(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	canonical, err := toolspkg.CanonicalJSON(raw)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(canonical), nil
}

func extensionProjectionRuntimeDescriptors(
	ctx context.Context,
	runtime ExtensionToolRuntime,
	key InstanceKey,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
		return scoped.ProvideToolsForInstance(ctx, key)
	}
	return runtime.ProvideTools(ctx, key.Name)
}

func appendExtensionProjectionFingerprint(
	dst *strings.Builder,
	key InstanceKey,
	snapshot *Extension,
) {
	dst.WriteString(key.runtimeID())
	dst.WriteByte(0)
	if snapshot == nil {
		dst.WriteString("missing")
		dst.WriteByte(0)
		return
	}
	state := struct {
		Enabled      bool
		Active       bool
		Healthy      bool
		Capabilities []string
	}{
		Enabled: snapshot.Info.Enabled,
		Active:  snapshot.Status.Active,
		Healthy: snapshot.Status.Healthy,
	}
	if snapshot.InitializeResult != nil {
		state.Capabilities = canonicalProjectionCapabilities(
			snapshot.InitializeResult.AcceptedCapabilities.Provides,
		)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		dst.WriteString(agentPluginInvalidStatus)
	} else {
		dst.Write(encoded)
	}
	dst.WriteByte(0)
}

func extensionProjectionGenerationToken(manifestFingerprint, runtimeFingerprint string) string {
	payload := strconv.Itoa(len(manifestFingerprint)) + ":" + manifestFingerprint +
		strconv.Itoa(len(runtimeFingerprint)) + ":" + runtimeFingerprint
	digest := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(digest[:])
}
