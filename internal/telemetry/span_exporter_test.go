package telemetry_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// UT-023
func TestSanitizingSpanExporterExportsErrorsAsClassOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should export MarkError as an empty description and a fixed class", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, testRegistries(nil))
		_, span := provider.Tracer(telemetry.ScopeTools).Start(context.Background(), telemetry.SpanToolExecute)
		telemetry.MarkError(span, errors.New("open /home/ana/secret.txt: permission denied"))
		span.End()

		got := onlySpan(t, exported)
		if got.Status.Code != codes.Error || got.Status.Description != "" {
			t.Fatalf("status = %+v, want Error with an empty description", got.Status)
		}
		if value := attributeMap(got.Attributes)[telemetry.AttrErrorType]; value.AsString() != telemetry.ErrorPermissionDenied {
			t.Fatalf("error.type = %q, want %q", value.AsString(), telemetry.ErrorPermissionDenied)
		}
	})

	t.Run("Should drop a status description set directly", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, testRegistries(nil))
		_, span := provider.Tracer(telemetry.ScopeTools).Start(context.Background(), telemetry.SpanToolExecute)
		span.SetStatus(codes.Error, "x")
		span.End()

		if got := onlySpan(t, exported).Status; got.Code != codes.Error || got.Description != "" {
			t.Fatalf("status = %+v, want Error with an empty description", got)
		}
	})
}

// UT-039
func TestSanitizingSpanExporterEnforcesNameAndScopeRegistries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		scope     string
		span      string
		wantName  string
		wantScope string
	}{
		{
			name:  "Should export an unregistered scope as compozy.unknown",
			scope: "example.com/unregistered", span: telemetry.SpanToolExecute,
			wantName: telemetry.SpanToolExecute, wantScope: telemetry.UnknownName,
		},
		{
			name:  "Should export an unregistered route template as compozy.unknown",
			scope: telemetry.ScopeHTTP, span: "GET /api/x/:unregistered",
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should keep the unmatched-route span name",
			scope: telemetry.ScopeHTTP, span: "HTTP GET",
			wantName: "HTTP GET", wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should keep a registered route template",
			scope: telemetry.ScopeHTTP, span: "POST " + testRoute,
			wantName: "POST " + testRoute, wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should keep a registered ACP method span",
			scope: telemetry.ScopeACP, span: "acp " + testACPMethod,
			wantName: "acp " + testACPMethod, wantScope: telemetry.ScopeACP,
		},
		{
			name:  "Should export an unregistered ACP method span as compozy.unknown",
			scope: telemetry.ScopeACP, span: "acp fs/read_text_file",
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeACP,
		},
		// An HTTP method is whatever token the client sent, so the method itself needs the registry check.
		{
			name:  "Should export an unmatched-route span with a non-method token as compozy.unknown",
			scope: telemetry.ScopeHTTP, span: "HTTP SECRET-CANARY-7f3a",
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should export an unmatched-route span with a lowercase method as compozy.unknown",
			scope: telemetry.ScopeHTTP, span: "HTTP get",
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should export an unmatched-route span carrying a path as compozy.unknown",
			scope: telemetry.ScopeHTTP, span: "HTTP GET /home/ana/secret.txt",
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeHTTP,
		},
		{
			name:  "Should export a registered route template under a non-method token as compozy.unknown",
			scope: telemetry.ScopeHTTP, span: "SECRET-CANARY-7f3a " + testRoute,
			wantName: telemetry.UnknownName, wantScope: telemetry.ScopeHTTP,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, testRegistries(nil))
			_, span := provider.Tracer(tc.scope, trace.WithInstrumentationAttributes(attribute.String("leak", "x"))).
				Start(context.Background(), tc.span)
			span.End()

			got := onlySpan(t, exported)
			if got.Name != tc.wantName {
				t.Fatalf("span name = %q, want %q", got.Name, tc.wantName)
			}
			scope := got.InstrumentationScope
			if scope.Name != tc.wantScope || scope.Version != testServiceVersion || scope.Attributes.Len() != 0 {
				t.Fatalf(
					"scope = %+v, want %q at version %q without attributes",
					scope,
					tc.wantScope,
					testServiceVersion,
				)
			}
		})
	}
}

// The rebuild keeps only what it copies, so a field it forgets exports as a different span: a lost parent breaks the
// trace tree, a lost kind or timestamp breaks the backend's latency view, a dropped registered event vanishes.
func TestSanitizingSpanExporterKeepsSpanIdentityKindAndTiming(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should export a span tree with its identity, kind, timing, and registered events unchanged",
		func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, testRegistries(nil))
			start := time.Unix(1_700_000_000, 0)
			firstToken, recovery, end := start.Add(
				150*time.Millisecond,
			), start.Add(
				time.Second,
			), start.Add(
				2*time.Second,
			)

			ctx, parent := provider.Tracer(telemetry.ScopeHTTP).Start(context.Background(), "POST "+testRoute,
				trace.WithSpanKind(trace.SpanKindServer), trace.WithTimestamp(start))
			_, child := provider.Tracer(telemetry.ScopeACP).Start(ctx, "acp "+testACPMethod,
				trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(start.Add(time.Millisecond)))
			child.AddEvent(telemetry.EventFirstToken, trace.WithTimestamp(firstToken))
			child.AddEvent(telemetry.EventRecovery, trace.WithTimestamp(recovery))
			child.End(trace.WithTimestamp(end.Add(-time.Millisecond)))
			parent.End(trace.WithTimestamp(end))

			byName := make(map[string]tracetest.SpanStub)
			for _, span := range exported.GetSpans() {
				byName[span.Name] = span
			}
			gotParent, gotChild := byName["POST "+testRoute], byName["acp "+testACPMethod]
			if len(byName) != 2 || gotParent.Name == "" || gotChild.Name == "" {
				t.Fatalf("exported spans = %v, want the HTTP span and its ACP child", slices.Sorted(maps.Keys(byName)))
			}

			if !gotParent.SpanContext.Equal(parent.SpanContext()) || gotParent.Parent.IsValid() {
				t.Fatalf("parent span context = %v parent %v, want %v and no parent",
					gotParent.SpanContext, gotParent.Parent, parent.SpanContext())
			}
			if !gotChild.SpanContext.Equal(child.SpanContext()) || !gotChild.Parent.Equal(parent.SpanContext()) {
				t.Fatalf("child span context = %v parent %v, want %v under %v",
					gotChild.SpanContext, gotChild.Parent, child.SpanContext(), parent.SpanContext())
			}
			if gotParent.SpanKind != trace.SpanKindServer || gotChild.SpanKind != trace.SpanKindClient {
				t.Fatalf("span kinds = %v and %v, want server and client", gotParent.SpanKind, gotChild.SpanKind)
			}
			if !gotParent.StartTime.Equal(start) || !gotParent.EndTime.Equal(end) {
				t.Fatalf("parent window = %v..%v, want %v..%v", gotParent.StartTime, gotParent.EndTime, start, end)
			}
			if !gotChild.StartTime.Equal(start.Add(time.Millisecond)) ||
				!gotChild.EndTime.Equal(end.Add(-time.Millisecond)) {
				t.Fatalf("child window = %v..%v, want %v..%v", gotChild.StartTime, gotChild.EndTime,
					start.Add(time.Millisecond), end.Add(-time.Millisecond))
			}
			if len(gotChild.Events) != 2 ||
				gotChild.Events[0].Name != telemetry.EventFirstToken || !gotChild.Events[0].Time.Equal(firstToken) ||
				gotChild.Events[1].Name != telemetry.EventRecovery || !gotChild.Events[1].Time.Equal(recovery) {
				t.Fatalf("child events = %+v, want %q at %v then %q at %v",
					gotChild.Events, telemetry.EventFirstToken, firstToken, telemetry.EventRecovery, recovery)
			}
		},
	)
}

// The batch processor hands the exporter many spans per call, so a rebuild that handled only some of them would
// lose data without any single-span case noticing: each span must keep its own identity and sanitized content.
func TestSanitizingSpanExporterRebuildsEverySpanOfABatch(t *testing.T) {
	t.Parallel()

	t.Run("Should export every span of one batch in order with its own sanitized content", func(t *testing.T) {
		t.Parallel()

		exported := tracetest.NewInMemoryExporter()
		exporter := telemetry.NewSanitizingSpanExporter(
			exported, telemetry.NewSanitizer(testRegistries(nil), newTestResource(), testServiceVersion))
		// Every span name below allows compozy.outcome, so the outcome tells the spans apart after the rebuild.
		spans := []struct {
			name     string
			wantName string
			id       byte
			outcome  string
		}{
			{name: telemetry.SpanSessionTurn, wantName: telemetry.SpanSessionTurn, id: 1, outcome: telemetry.OutcomeOK},
			{name: "prompt: hello", wantName: telemetry.UnknownName, id: 2, outcome: telemetry.OutcomeError},
			{
				name: telemetry.SpanToolExecute, wantName: telemetry.SpanToolExecute, id: 3,
				outcome: telemetry.OutcomeCancelled,
			},
			{name: "acp " + testACPMethod, wantName: "acp " + testACPMethod, id: 4, outcome: telemetry.OutcomeApproved},
			{name: telemetry.SpanDBWrite, wantName: telemetry.SpanDBWrite, id: 5, outcome: telemetry.OutcomeDenied},
			{
				name: telemetry.SpanHookDispatch, wantName: telemetry.SpanHookDispatch, id: 6,
				outcome: telemetry.OutcomeUnbound,
			},
		}
		batch := make(tracetest.SpanStubs, 0, len(spans))
		for _, span := range spans {
			batch = append(batch, tracetest.SpanStub{
				Name: span.name,
				SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: trace.TraceID{1}, SpanID: trace.SpanID{span.id}, TraceFlags: trace.FlagsSampled,
				}),
				Attributes: []attribute.KeyValue{
					telemetry.AttrOutcome.String(span.outcome), attribute.String("prompt.text", "SECRET-CANARY-7f3a"),
				},
			})
		}

		if err := exporter.ExportSpans(context.Background(), batch.Snapshots()); err != nil {
			t.Fatalf("ExportSpans() error = %v", err)
		}

		got := exported.GetSpans()
		if len(got) != len(spans) {
			t.Fatalf("exported %d spans of a batch of %d, want every span", len(got), len(spans))
		}
		for index, want := range spans {
			span, attrs := got[index], attributeMap(got[index].Attributes)
			if span.Name != want.wantName || span.SpanContext.SpanID() != (trace.SpanID{want.id}) ||
				len(attrs) != 1 || attrs[telemetry.AttrOutcome].AsString() != want.outcome {
				t.Errorf("span %d = %q id %v attributes %v, want %q id %v with only compozy.outcome=%q",
					index, span.Name, span.SpanContext.SpanID(), span.Attributes, want.wantName,
					trace.SpanID{want.id}, want.outcome)
			}
		}
	})
}

// UT-046 (spans; the metric half is in metric_exporter_test.go)
func TestSanitizingSpanExporterAppliesPerKeyFallbackPolicy(t *testing.T) {
	t.Parallel()

	secret := "telemetry-span-secret-41c7"
	t.Cleanup(redact.RegisterRequiredSecret(secret))
	secretModel := "org/model-" + secret
	reg := testRegistries(nil, secretModel)

	for _, tc := range modelPolicyCases(secretModel) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, exported := newTracerProvider(t, reg)
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), telemetry.SpanSessionTurn,
				trace.WithAttributes(telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(tc.model)),
			)
			span.End()

			if got := attributeMap(onlySpan(t, exported).Attributes)[telemetry.AttrGenAIModel]; got.AsString() != tc.want {
				t.Fatalf("gen_ai.request.model(%q) = %q, want %q", tc.model, got.AsString(), tc.want)
			}
		})
	}

	t.Run("Should replace unregistered agents and providers and omit invalid identities and enums", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, reg)
		_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), telemetry.SpanSessionTurn,
			trace.WithAttributes(
				telemetry.AttrAgent.String("unregistered-agent"),
				telemetry.AttrGenAISystem.String("unregistered-provider"),
				telemetry.AttrSessionID.String("/home/ana/session"),
				telemetry.AttrOutcome.String("exploded"),
			))
		span.End()

		got := attributeMap(onlySpan(t, exported).Attributes)
		want := map[attribute.Key]attribute.Value{
			telemetry.AttrAgent:       attribute.StringValue(telemetry.ValueOther),
			telemetry.AttrGenAISystem: attribute.StringValue(telemetry.ValueOther),
		}
		if len(got) != len(want) || got[telemetry.AttrAgent] != want[telemetry.AttrAgent] ||
			got[telemetry.AttrGenAISystem] != want[telemetry.AttrGenAISystem] {
			t.Fatalf("attributes = %v, want %v", got, want)
		}
	})

	// A model is only meaningful under its provider (the model catalog is keyed by both), so the exporter must hand
	// the span's own gen_ai.system to the registry; otherwise every catalog model would silently export as other.
	t.Run("Should classify the model under the provider recorded on the same span", func(t *testing.T) {
		t.Parallel()

		const providerBoundModel = "org/provider-bound-model"
		reg := testRegistries(nil)
		reg.Models = func(provider, model string) telemetry.ModelClass {
			if provider == testProvider && model == providerBoundModel {
				return telemetry.ModelCatalog
			}
			return telemetry.ModelUnknown
		}
		cases := []struct {
			name  string
			attrs []attribute.KeyValue
			want  string
		}{
			{
				name: "Should keep the model under its provider",
				attrs: []attribute.KeyValue{
					telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(providerBoundModel),
				},
				want: providerBoundModel,
			},
			{
				name: "Should export the model as other under another provider",
				attrs: []attribute.KeyValue{
					telemetry.AttrGenAISystem.String("unregistered-provider"),
					telemetry.AttrGenAIModel.String(providerBoundModel),
				},
				want: telemetry.ValueOther,
			},
			{
				name:  "Should export the model as other without a provider",
				attrs: []attribute.KeyValue{telemetry.AttrGenAIModel.String(providerBoundModel)},
				want:  telemetry.ValueOther,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				provider, exported := newTracerProvider(t, reg)
				_, span := provider.Tracer(telemetry.ScopeSession).
					Start(context.Background(), telemetry.SpanSessionTurn,
						trace.WithAttributes(tc.attrs...))
				span.End()

				if got := attributeMap(onlySpan(t, exported).Attributes)[telemetry.AttrGenAIModel]; got.AsString() != tc.want {
					t.Fatalf("gen_ai.request.model = %q, want %q", got.AsString(), tc.want)
				}
			})
		}
	})

	// ModelClass documents that its zero value is ModelUnknown, so a registry that returns a class it never set
	// (an unset variable, a failed lookup) must export other and never the model name.
	t.Run("Should export a model as other when the registry returns the zero class", func(t *testing.T) {
		t.Parallel()

		reg := testRegistries(nil)
		reg.Models = func(string, string) telemetry.ModelClass {
			var unset telemetry.ModelClass
			return unset
		}
		provider, exported := newTracerProvider(t, reg)
		_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), telemetry.SpanSessionTurn,
			trace.WithAttributes(
				telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(testCatalogModel)))
		span.End()

		got := attributeMap(onlySpan(t, exported).Attributes)[telemetry.AttrGenAIModel]
		if got.AsString() != telemetry.ValueOther {
			t.Fatalf("gen_ai.request.model = %q, want %q for the zero ModelClass", got.AsString(), telemetry.ValueOther)
		}
	})
}

type modelPolicyCase struct {
	name  string
	model string
	want  string
}

func modelPolicyCases(secretModel string) []modelPolicyCase {
	cases := []modelPolicyCase{
		{
			name:  "Should export an unregistered path model as other",
			model: "/home/ana/secret.txt",
			want:  telemetry.ValueOther,
		},
		{
			name:  "Should export an unregistered canary model as other",
			model: "SECRET-CANARY-7f3a",
			want:  telemetry.ValueOther,
		},
		{
			name: "Should export an unregistered hex model as other", model: strings.Repeat("ab", 32),
			want: telemetry.ValueOther,
		},
		{name: "Should export a catalog model with a slash unchanged", model: testCatalogModel, want: testCatalogModel},
		{
			name: "Should export a registered model containing a registered secret as invalid", model: secretModel,
			want: telemetry.ValueInvalid,
		},
	}
	for _, model := range testConfiguredModels {
		cases = append(cases, modelPolicyCase{
			name:  "Should export the configuration-only model " + model + " as custom",
			model: model,
			want:  telemetry.ValueCustom,
		})
	}
	return cases
}

// UT-047
func TestSanitizingSpanExporterKeepsOwnerIdentities(t *testing.T) {
	t.Parallel()

	longAgents := []string{"a" + strings.Repeat("b", 99), "a" + strings.Repeat("c", 99)}
	cases := ownerIdentityCases(t, longAgents)
	reg := testRegistries(longAgents)

	t.Run("Should export every owner identity unchanged and distinct", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, reg)
		for _, tc := range cases {
			_, span := provider.Tracer(telemetry.ScopeSession).Start(context.Background(), tc.span,
				trace.WithAttributes(tc.key.String(tc.value)))
			span.End()
		}

		spans := exported.GetSpans()
		if len(spans) != len(cases) {
			t.Fatalf("exported %d spans, want %d", len(spans), len(cases))
		}
		distinct := make(map[string]struct{}, len(cases))
		for index, tc := range cases {
			got, ok := attributeMap(spans[index].Attributes)[tc.key]
			if !ok || got.AsString() != tc.value {
				t.Errorf("%s: %s = %q (present %t), want %q unchanged", tc.name, tc.key, got.AsString(), ok, tc.value)
			}
			distinct[got.AsString()] = struct{}{}
		}
		if len(distinct) != len(cases) {
			t.Fatalf("exported %d distinct identities, want %d", len(distinct), len(cases))
		}
	})

	t.Run("Should omit a free-text automation id", func(t *testing.T) {
		t.Parallel()

		provider, exported := newTracerProvider(t, reg)
		_, span := provider.Tracer(telemetry.ScopeAutomation).Start(context.Background(), telemetry.SpanAutomationRun,
			trace.WithAttributes(telemetry.AttrAutomationID.String("nightly report for ana")))
		span.End()

		if attrs := onlySpan(t, exported).Attributes; len(attrs) != 0 {
			t.Fatalf("attributes = %v, want the free-text automation id omitted", attrs)
		}
	})
}

type ownerIdentityCase struct {
	name  string
	span  string
	key   attribute.Key
	value string
}

func ownerIdentityCases(t *testing.T, longAgents []string) []ownerIdentityCase {
	t.Helper()
	newID := func(prefix string) string {
		t.Helper()
		id, err := store.NewID(prefix)
		if err != nil {
			t.Fatalf("store.NewID(%q) error = %v", prefix, err)
		}
		return id
	}
	workspaceID, err := workspace.NewWorkspaceID()
	if err != nil {
		t.Fatalf("workspace.NewWorkspaceID() error = %v", err)
	}
	_, bindingSessionID := goal.DeriveBindingIdentity(goal.TurnKey{
		LoopRunID: "looprun-0123456789abcdef", WorkspaceID: "ws_0123456789abcdef", Generation: 1, NodeID: "build",
	}, "handle", 1)
	// Subagent and restart-continuation ids mirror internal/session's private derivations.
	subagentSessionID := "sess-" + strings.TrimPrefix(subagentid.Derive("sess-0123456789abcdef", "tool-call-1"), "sub-")
	restartDigest := sha256.Sum256([]byte("turn-0123456789abcdef:continue"))
	restartTurnID := "turn-ev_restart_" + hex.EncodeToString(restartDigest[:16])

	turn := telemetry.SpanSessionTurn
	return []ownerIdentityCase{
		{
			name:  "Should keep the default profile",
			span:  turn,
			key:   telemetry.AttrProfileID,
			value: store.DefaultProfileID,
		},
		{
			name:  "Should keep a generated profile",
			span:  turn,
			key:   telemetry.AttrProfileID,
			value: ulid.Make().String(),
		},
		{name: "Should keep a workspace ULID", span: turn, key: telemetry.AttrWorkspaceID, value: workspaceID},
		{
			name:  "Should keep a legacy workspace id",
			span:  turn,
			key:   telemetry.AttrWorkspaceID,
			value: "ws_35748d1db3d1f9ce",
		},
		{name: "Should keep a generated session", span: turn, key: telemetry.AttrSessionID, value: newID("sess")},
		{name: "Should keep a subagent session", span: turn, key: telemetry.AttrSessionID, value: subagentSessionID},
		{name: "Should keep a Loop binding session", span: turn, key: telemetry.AttrSessionID, value: bindingSessionID},
		{name: "Should keep a generated turn", span: turn, key: telemetry.AttrTurnID, value: newID("turn")},
		{name: "Should keep a restart continuation turn", span: turn, key: telemetry.AttrTurnID, value: restartTurnID},
		{name: "Should keep a generated run", span: turn, key: telemetry.AttrRunID, value: newID("run")},
		{name: "Should keep a first long agent", span: turn, key: telemetry.AttrAgent, value: longAgents[0]},
		{name: "Should keep a second long agent", span: turn, key: telemetry.AttrAgent, value: longAgents[1]},
		{
			name:  "Should keep a generated Loop run",
			span:  telemetry.SpanLoopRun,
			key:   telemetry.AttrLoopID,
			value: newID("looprun"),
		},
		{
			name: "Should keep a generated automation job", span: telemetry.SpanAutomationRun,
			key: telemetry.AttrAutomationID, value: newID("job"),
		},
	}
}
