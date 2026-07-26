package resources_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/resources"
)

type externalSpec struct {
	Name string `json:"name"`
}

type externalCodec struct {
	kind resources.ResourceKind
}

var _ resources.KindCodec[externalSpec] = (*externalCodec)(nil)

func (c *externalCodec) Kind() resources.ResourceKind {
	return c.kind
}

func (c *externalCodec) DecodeAndValidate(
	ctx context.Context,
	_ resources.ResourceScope,
	raw []byte,
) (externalSpec, error) {
	if ctx == nil {
		return externalSpec{}, errors.New("external codec context is required")
	}

	var spec externalSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return externalSpec{}, fmt.Errorf("external codec decode: %w", err)
	}
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return externalSpec{}, errors.New("external codec name is required")
	}
	return spec, nil
}

func (c *externalCodec) Encode(spec externalSpec) ([]byte, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("external codec encode: %w", err)
	}
	return encoded, nil
}

func (c *externalCodec) MaxBytes() int {
	return 1024
}

func TestCodecRegistryExternalContract(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize codecs registered through the exported contract", func(t *testing.T) {
		t.Parallel()

		kind := resources.ResourceKind("external.custom")
		registry := resources.NewCodecRegistry()
		if err := resources.RegisterCodec[externalSpec](registry, &externalCodec{kind: kind}); err != nil {
			t.Fatalf("RegisterCodec() error = %v", err)
		}

		canonical, validated, err := resources.ValidateAndCanonicalizeIfRegistered(
			context.Background(),
			registry,
			kind,
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			[]byte(`{"name":"  alpha  "}`),
		)
		if err != nil {
			t.Fatalf("ValidateAndCanonicalizeIfRegistered() error = %v", err)
		}
		if !validated {
			t.Fatal("ValidateAndCanonicalizeIfRegistered() validated = false, want true")
		}
		if got, want := string(canonical), `{"name":"alpha"}`; got != want {
			t.Fatalf("canonical payload = %s, want %s", got, want)
		}
	})
}
