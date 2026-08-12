package extensionpkg

import (
	"context"
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
	manifestTools, err := p.manifestTools(scope)
	if err != nil {
		return "", false
	}
	runtimeFingerprint, known := p.runtimeProjectionFingerprint(ctx, manifestTools)
	if !known {
		return "", false
	}
	return p.updateProjectionGeneration(runtimeFingerprint), true
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
		encoded, err := json.Marshal(descriptors)
		if err != nil {
			return "", false
		}
		fingerprint.Write(encoded)
		fingerprint.WriteByte(0)
	}
	return fingerprint.String(), true
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
		state.Capabilities = slices.Clone(snapshot.InitializeResult.AcceptedCapabilities.Provides)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		dst.WriteString("invalid")
	} else {
		dst.Write(encoded)
	}
	dst.WriteByte(0)
}

func (p *ExtensionToolProvider) updateProjectionGeneration(runtimeFingerprint string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache.runtimeFingerprint != runtimeFingerprint {
		p.cache.runtimeFingerprint = runtimeFingerprint
		p.cache.generation++
	}
	if p.cache.generation == 0 {
		p.cache.generation = 1
	}
	return strconv.FormatUint(p.cache.generation, 10)
}
