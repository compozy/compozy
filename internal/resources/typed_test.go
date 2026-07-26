package resources

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

type testTypedSpec struct {
	Name string `json:"name"`
}

type otherTypedSpec struct {
	Value string `json:"value"`
}

type testPlan struct {
	kind       ResourceKind
	revision   int64
	operations int
}

func (p testPlan) Kind() ResourceKind {
	return p.kind
}

func (p testPlan) Revision() int64 {
	return p.revision
}

func (p testPlan) OperationCount() int {
	return p.operations
}

type countingCodec[T any] struct {
	inner       KindCodec[T]
	decodeCount int
	encodeCount int
}

func (c *countingCodec[T]) Kind() ResourceKind {
	return c.inner.Kind()
}

func (c *countingCodec[T]) DecodeAndValidate(ctx context.Context, scope ResourceScope, raw []byte) (T, error) {
	c.decodeCount++
	return c.inner.DecodeAndValidate(ctx, scope, raw)
}

func (c *countingCodec[T]) Encode(spec T) ([]byte, error) {
	c.encodeCount++
	return c.inner.Encode(spec)
}

func (c *countingCodec[T]) MaxBytes() int {
	return c.inner.MaxBytes()
}

type captureTypedProjector struct {
	kind       ResourceKind
	dependsOn  []ResourceKind
	buildCalls int
	records    []Record[testTypedSpec]
	applied    ProjectionPlan
}

func (p *captureTypedProjector) Kind() ResourceKind {
	return p.kind
}

func (p *captureTypedProjector) DependsOn() []ResourceKind {
	return append([]ResourceKind(nil), p.dependsOn...)
}

func (p *captureTypedProjector) Build(
	_ context.Context,
	records []Record[testTypedSpec],
) (ProjectionPlan, error) {
	p.buildCalls++
	p.records = append([]Record[testTypedSpec](nil), records...)
	return testPlan{
		kind:       p.kind,
		revision:   7,
		operations: len(records),
	}, nil
}

func (p *captureTypedProjector) Apply(_ context.Context, plan ProjectionPlan) error {
	p.applied = plan
	return nil
}

type captureBundleActivationProjector struct {
	buildCalls  int
	activations []Record[testTypedSpec]
	bundles     []Record[otherTypedSpec]
	applied     ProjectionPlan
}

func (p *captureBundleActivationProjector) Build(
	_ context.Context,
	activations []Record[testTypedSpec],
	bundles []Record[otherTypedSpec],
) (ProjectionPlan, error) {
	p.buildCalls++
	p.activations = append([]Record[testTypedSpec](nil), activations...)
	p.bundles = append([]Record[otherTypedSpec](nil), bundles...)
	return testPlan{
		kind:       bundleActivationKind,
		revision:   11,
		operations: len(activations) + len(bundles),
	}, nil
}

func (p *captureBundleActivationProjector) Apply(_ context.Context, plan ProjectionPlan) error {
	p.applied = plan
	return nil
}

func TestCodecRegistryRegistrationAndResolve(t *testing.T) {
	t.Parallel()

	registry := NewCodecRegistry()
	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)

	if err := RegisterCodec(registry, codec); err != nil {
		t.Fatalf("RegisterCodec() error = %v", err)
	}
	if err := RegisterCodec(registry, codec); !errors.Is(err, ErrConflict) {
		t.Fatalf("RegisterCodec(duplicate) error = %v, want ErrConflict", err)
	}

	resolved, err := ResolveCodec[testTypedSpec](registry, testResourceKind)
	if err != nil {
		t.Fatalf("ResolveCodec(testTypedSpec) error = %v", err)
	}
	if got, want := resolved.Kind(), testResourceKind; got != want {
		t.Fatalf("resolved.Kind() = %q, want %q", got, want)
	}

	if _, err := ResolveCodec[otherTypedSpec](registry, testResourceKind); !errors.Is(err, ErrCodecTypeMismatch) {
		t.Fatalf("ResolveCodec(type mismatch) error = %v, want ErrCodecTypeMismatch", err)
	}
	if _, err := ResolveCodec[testTypedSpec](registry, ResourceKind("missing")); !errors.Is(err, ErrCodecNotFound) {
		t.Fatalf("ResolveCodec(missing) error = %v, want ErrCodecNotFound", err)
	}
}

func TestJSONCodecValidationClassification(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		input           []byte
		validatorErr    error
		wantErr         error
		wantErrContains string
		wantErrExcludes error
	}{
		{
			name:            "Should wrap plain validator errors as resource validation",
			input:           []byte("{\"name\":\"\"}"),
			validatorErr:    errors.New("name is required"),
			wantErr:         ErrValidation,
			wantErrContains: "name is required",
		},
		{
			name:            "Should preserve classified validator errors",
			input:           []byte("{\"name\":\"oversized\"}"),
			validatorErr:    ErrPayloadTooLarge,
			wantErr:         ErrPayloadTooLarge,
			wantErrExcludes: ErrValidation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			codec := mustJSONCodec(
				t,
				testResourceKind,
				1024,
				func(context.Context, ResourceScope, testTypedSpec) (testTypedSpec, error) {
					return testTypedSpec{}, tc.validatorErr
				},
			)

			_, err := codec.DecodeAndValidate(
				testutil.Context(t),
				ResourceScope{Kind: ResourceScopeKindGlobal},
				tc.input,
			)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("DecodeAndValidate() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Fatalf("DecodeAndValidate() error = %v, want validator context %q", err, tc.wantErrContains)
			}
			if tc.wantErrExcludes != nil && errors.Is(err, tc.wantErrExcludes) {
				t.Fatalf("DecodeAndValidate() error = %v, should not include %v", err, tc.wantErrExcludes)
			}
		})
	}
}

func TestTypedStoreReadAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)

	t.Run("Should foreign source get denied and list filtered", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		store, err := NewStore(kernel, codec)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}

		ctx := testutil.Context(t)
		sourceAlpha := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-alpha"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), sourceAlpha, "nonce-alpha"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}
		if err := kernel.ApplySourceSnapshotRaw(
			ctx,
			testExtensionActor("session-alpha", sourceAlpha.ID, "nonce-alpha"),
			SourceSnapshot{
				SourceVersion: 1,
				Records: []RawDraft{{
					Kind:            testResourceKind,
					ID:              "foreign-tool",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"name":"alpha"}`),
				}},
			},
		); err != nil {
			t.Fatalf("ApplySourceSnapshotRaw() error = %v", err)
		}

		foreignActor := testExtensionActor("session-bravo", "ext-bravo", "nonce-bravo")
		if _, err := store.Get(ctx, foreignActor, "foreign-tool"); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("Get(foreign source) error = %v, want ErrPermissionDenied", err)
		}

		records, err := store.List(ctx, foreignActor, ResourceFilter{})
		if err != nil {
			t.Fatalf("List(foreign source) error = %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("List(foreign source) = %#v, want none", records)
		}
	})

	t.Run("Should granted kind mismatch rejected", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		store, err := NewStore(kernel, codec)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}

		ctx := testutil.Context(t)
		record, err := store.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
			ID:    "kind-check",
			Scope: ResourceScope{Kind: ResourceScopeKindGlobal},
			Spec:  testTypedSpec{Name: "alpha"},
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		actor := testExtensionActor("session-kind", "ext-kind", "nonce-kind")
		actor.GrantedKinds = []ResourceKind{ResourceKind("other.kind")}
		actor.GrantedScopes = []ResourceScopeKind{ResourceScopeKindGlobal}

		if _, err := store.Get(ctx, actor, record.ID); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("Get(denied kind) error = %v, want ErrPermissionDenied", err)
		}
		if _, err := store.List(ctx, actor, ResourceFilter{}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("List(denied kind) error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should scope boundary rejected", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		store, err := NewStore(kernel, codec)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}

		ctx := testutil.Context(t)
		record, err := store.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
			ID:    "scope-check",
			Scope: ResourceScope{Kind: ResourceScopeKindGlobal},
			Spec:  testTypedSpec{Name: "alpha"},
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		workspaceActor := testOperatorActor()
		workspaceActor.MaxScope = ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}
		workspaceActor.GrantedKinds = []ResourceKind{testResourceKind}
		workspaceActor.GrantedScopes = []ResourceScopeKind{ResourceScopeKindWorkspace}

		if _, err := store.Get(ctx, workspaceActor, record.ID); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("Get(scope boundary) error = %v, want ErrPermissionDenied", err)
		}
		if _, err := store.List(ctx, workspaceActor, ResourceFilter{
			Scope: &ResourceScope{Kind: ResourceScopeKindGlobal},
		}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("List(scope boundary) error = %v, want ErrPermissionDenied", err)
		}
	})
}

func TestTypedStoreDecodeFailureRejectsInvalidRawPayloads(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)
	store, err := NewStore(kernel, codec)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if _, err := kernel.PutRaw(ctx, testDaemonActor(), RawDraft{
		Kind:            testResourceKind,
		ID:              "decode-failure",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"   "}`),
	}); err != nil {
		t.Fatalf("PutRaw() error = %v", err)
	}

	if _, err := store.Get(ctx, testDaemonActor(), "decode-failure"); !errors.Is(err, ErrValidation) {
		t.Fatalf("Get() error = %v, want ErrValidation", err)
	}
}

func TestTypedStorePutRoundTripPreservesMetadata(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)
	store, err := NewStore(kernel, codec)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	record, err := store.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
		ID:    "typed-round-trip",
		Scope: ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-42"},
		Spec:  testTypedSpec{Name: "  alpha  "},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if got, want := record.Kind, testResourceKind; got != want {
		t.Fatalf("record.Kind = %q, want %q", got, want)
	}
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("record.Version = %d, want %d", got, want)
	}
	if got, want := record.Scope, (ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-42"}); got != want {
		t.Fatalf("record.Scope = %#v, want %#v", got, want)
	}
	if got, want := record.Owner, (ResourceOwner{Kind: ResourceOwnerKind("daemon"), ID: "daemon-control"}); got != want {
		t.Fatalf("record.Owner = %#v, want %#v", got, want)
	}
	if got, want := record.Source, (ResourceSource{Kind: ResourceSourceKind("daemon"), ID: "system"}); got != want {
		t.Fatalf("record.Source = %#v, want %#v", got, want)
	}
	if got, want := record.Spec.Name, "alpha"; got != want {
		t.Fatalf("record.Spec.Name = %q, want %q", got, want)
	}

	loaded, err := store.Get(ctx, testDaemonActor(), record.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := loaded.Version, record.Version; got != want {
		t.Fatalf("loaded.Version = %d, want %d", got, want)
	}
	if got, want := loaded.Owner, record.Owner; got != want {
		t.Fatalf("loaded.Owner = %#v, want %#v", got, want)
	}
	if got, want := loaded.Source, record.Source; got != want {
		t.Fatalf("loaded.Source = %#v, want %#v", got, want)
	}
}

func TestTypedProjectorRegistrationDecodesPrimaryKindOnce(t *testing.T) {
	t.Parallel()

	baseCodec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)
	codec := &countingCodec[testTypedSpec]{inner: baseCodec}
	domainProjector := &captureTypedProjector{
		kind: testResourceKind,
	}

	registration, err := NewTypedProjectorRegistration(codec, domainProjector)
	if err != nil {
		t.Fatalf("NewTypedProjectorRegistration() error = %v", err)
	}

	internalProjector, err := unwrapProjectorRegistration(registration)
	if err != nil {
		t.Fatalf("unwrapProjectorRegistration() error = %v", err)
	}

	plan, err := internalProjector.Build(testutil.Context(t), projectionInput{
		kind: testResourceKind,
		records: []RawRecord{
			{
				Kind:     testResourceKind,
				ID:       "alpha",
				Scope:    ResourceScope{Kind: ResourceScopeKindGlobal},
				SpecJSON: []byte(`{"name":"alpha"}`),
			},
			{
				Kind:     testResourceKind,
				ID:       "beta",
				Scope:    ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"},
				SpecJSON: []byte(`{"name":"beta"}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := codec.decodeCount, 2; got != want {
		t.Fatalf("codec.decodeCount = %d, want %d", got, want)
	}
	if got, want := domainProjector.buildCalls, 1; got != want {
		t.Fatalf("buildCalls = %d, want %d", got, want)
	}
	if got, want := len(domainProjector.records), 2; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if got, want := plan.OperationCount(), 2; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}

	if err := internalProjector.Apply(testutil.Context(t), plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if domainProjector.applied == nil {
		t.Fatal("Apply() did not reach domain projector")
	}
}

func TestBundleActivationProjectorRegistrationDecodesDependenciesExplicitly(t *testing.T) {
	t.Parallel()

	registry := NewCodecRegistry()
	activationCodec := &countingCodec[testTypedSpec]{
		inner: mustJSONCodec(t, bundleActivationKind, 1024, validateTestTypedSpec),
	}
	bundleCodec := &countingCodec[otherTypedSpec]{
		inner: mustJSONCodec(t, bundleKind, 1024, validateOtherTypedSpec),
	}
	if err := RegisterCodec(registry, activationCodec); err != nil {
		t.Fatalf("RegisterCodec(activation) error = %v", err)
	}
	if err := RegisterCodec(registry, bundleCodec); err != nil {
		t.Fatalf("RegisterCodec(bundle) error = %v", err)
	}

	domainProjector := &captureBundleActivationProjector{}
	registration, err := NewBundleActivationProjectorRegistration(registry, domainProjector)
	if err != nil {
		t.Fatalf("NewBundleActivationProjectorRegistration() error = %v", err)
	}
	internalProjector, err := unwrapProjectorRegistration(registration)
	if err != nil {
		t.Fatalf("unwrapProjectorRegistration() error = %v", err)
	}

	plan, err := internalProjector.Build(testutil.Context(t), projectionInput{
		kind: bundleActivationKind,
		records: []RawRecord{{
			Kind:     bundleActivationKind,
			ID:       "activation-1",
			Scope:    ResourceScope{Kind: ResourceScopeKindGlobal},
			SpecJSON: []byte(`{"name":"activation-1"}`),
		}},
		dependencies: map[ResourceKind][]RawRecord{
			bundleKind: {{
				Kind:     bundleKind,
				ID:       "bundle-1",
				Scope:    ResourceScope{Kind: ResourceScopeKindGlobal},
				SpecJSON: []byte(`{"value":"bundle-1"}`),
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := activationCodec.decodeCount, 1; got != want {
		t.Fatalf("activationCodec.decodeCount = %d, want %d", got, want)
	}
	if got, want := bundleCodec.decodeCount, 1; got != want {
		t.Fatalf("bundleCodec.decodeCount = %d, want %d", got, want)
	}
	if got, want := len(domainProjector.activations), 1; got != want {
		t.Fatalf("len(activations) = %d, want %d", got, want)
	}
	if got, want := len(domainProjector.bundles), 1; got != want {
		t.Fatalf("len(bundles) = %d, want %d", got, want)
	}
	if got, want := plan.Kind(), bundleActivationKind; got != want {
		t.Fatalf("plan.Kind() = %q, want %q", got, want)
	}

	if err := internalProjector.Apply(testutil.Context(t), plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if domainProjector.applied == nil {
		t.Fatal("Apply() did not reach bundle activation projector")
	}

	_, err = internalProjector.Build(testutil.Context(t), projectionInput{
		kind: bundleActivationKind,
		dependencies: map[ResourceKind][]RawRecord{
			ResourceKind("automation.job"): {{
				Kind:     ResourceKind("automation.job"),
				ID:       "job-1",
				Scope:    ResourceScope{Kind: ResourceScopeKindGlobal},
				SpecJSON: []byte(`{"name":"job-1"}`),
			}},
		},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Build(unexpected dependency kind) error = %v, want ErrValidation", err)
	}
}

func TestTypedContractsDoNotExposeJSONRawMessage(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	targets := map[string]bool{
		"SpecValidator":             false,
		"TypedProjector":            false,
		"BundleActivationProjector": false,
	}
	found := make(map[string]bool, len(targets))

	for _, name := range []string{"codec.go", "projector.go"} {
		path := filepath.Join(".", name)
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := targets[typeSpec.Name.Name]; !ok {
					continue
				}
				found[typeSpec.Name.Name] = true
				if containsJSONRawMessage(typeSpec.Type) {
					t.Fatalf("%s must not expose json.RawMessage directly", typeSpec.Name.Name)
				}
			}
		}
	}

	for name := range targets {
		if !found[name] {
			t.Fatalf("type %s not found in package AST", name)
		}
	}
}

func containsJSONRawMessage(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(next ast.Node) bool {
		if found {
			return false
		}
		selector, ok := next.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgName, ok := selector.X.(*ast.Ident)
		if ok && pkgName.Name == "json" && selector.Sel.Name == "RawMessage" {
			found = true
			return false
		}
		return true
	})
	return found
}

func mustJSONCodec[T any](
	t testing.TB,
	kind ResourceKind,
	maxBytes int,
	validator SpecValidator[T],
) KindCodec[T] {
	t.Helper()

	codec, err := NewJSONCodec(kind, maxBytes, validator)
	if err != nil {
		t.Fatalf("NewJSONCodec() error = %v", err)
	}
	return codec
}

func validateTestTypedSpec(_ context.Context, _ ResourceScope, spec testTypedSpec) (testTypedSpec, error) {
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return testTypedSpec{}, fmt.Errorf("%w: test spec name is required", ErrValidation)
	}
	return spec, nil
}

func validateOtherTypedSpec(_ context.Context, _ ResourceScope, spec otherTypedSpec) (otherTypedSpec, error) {
	spec.Value = strings.TrimSpace(spec.Value)
	if spec.Value == "" {
		return otherTypedSpec{}, fmt.Errorf("%w: other spec value is required", ErrValidation)
	}
	return spec, nil
}
