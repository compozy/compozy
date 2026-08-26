package contracts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type memoryRegistryStore struct {
	mu        sync.Mutex
	contracts map[string]Contract
}

func newMemoryRegistryStore() *memoryRegistryStore {
	return &memoryRegistryStore{contracts: make(map[string]Contract)}
}

func (s *memoryRegistryStore) PutContract(_ context.Context, contract Contract) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, exists := s.contracts[contract.Digest]; exists &&
		string(current.Schema) != string(contract.Schema) {
		return errors.New("immutable contract conflict")
	}
	s.contracts[contract.Digest] = Contract{
		Digest: contract.Digest,
		Schema: cloneRaw(contract.Schema),
	}
	return nil
}

func (s *memoryRegistryStore) GetContract(_ context.Context, digest string) (Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[digest]
	if !ok {
		return Contract{}, ErrContractNotFound
	}
	contract.Schema = cloneRaw(contract.Schema)
	return contract, nil
}

func TestRegistryContractLifecycle(t *testing.T) {
	t.Parallel()

	store := newMemoryRegistryStore()
	registry := NewRegistry(store)
	ctx := context.Background()

	t.Run("Should pin canonical schemas and resolve identical bytes", func(t *testing.T) {
		t.Parallel()

		first, err := registry.Pin(ctx, json.RawMessage(`{
			"type":"object",
			"properties":{"name":{"type":"string"}},
			"required":["name"]
		}`))
		if err != nil {
			t.Fatalf("Pin(first) error = %v", err)
		}
		second, err := registry.Pin(ctx, json.RawMessage(`{
			"required":["name"],
			"properties":{"name":{"type":"string"}},
			"type":"object"
		}`))
		if err != nil {
			t.Fatalf("Pin(second) error = %v", err)
		}
		if first.Digest != second.Digest {
			t.Fatalf("Digest mismatch: %q != %q", first.Digest, second.Digest)
		}
		if !strings.HasPrefix(first.Digest, "sha256:") || len(first.Digest) != 71 {
			t.Fatalf("Digest = %q, want sha256:<64hex>", first.Digest)
		}
		resolved, err := registry.Resolve(ctx, first.Digest)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if string(resolved.Schema) != string(first.Schema) {
			t.Fatalf("Resolve().Schema = %s, want %s", resolved.Schema, first.Schema)
		}
	})

	t.Run("Should return a typed error for an unknown digest", func(t *testing.T) {
		t.Parallel()

		_, err := registry.Resolve(ctx, "sha256:missing")
		if !IsCode(err, CodeContractNotFound) {
			t.Fatalf("Resolve() error = %v, want %s", err, CodeContractNotFound)
		}
	})

	t.Run("Should reject invalid and non-object contract roots", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name   string
			schema json.RawMessage
		}{
			{name: "Should reject an array root", schema: json.RawMessage(`["not","an","object"]`)},
			{name: "Should reject a non-object schema root", schema: json.RawMessage(`{"type":"array","items":{"type":"string"}}`)},
			{name: "Should reject an invalid property type", schema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"not-a-type"}}}`)},
			{name: "Should reject trailing JSON values", schema: json.RawMessage(`{} {}`)},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				_, err := registry.Pin(ctx, test.schema)
				if !IsCode(err, CodeExpectInvalid) {
					t.Fatalf("Pin(%s) error = %v, want %s", test.schema, err, CodeExpectInvalid)
				}
			})
		}
	})
}

func TestRegistryValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemoryRegistryStore()
	registry := NewRegistry(store)
	contract, err := registry.Pin(ctx, json.RawMessage(`{
		"type":"object",
		"properties":{"findings":{"type":"array","items":{"type":"object","properties":{
			"severity":{"type":"string"},"note":{"type":"string"}
		},"required":["severity","note"]}}},
		"required":["findings"]
	}`))
	if err != nil {
		t.Fatalf("Pin() error = %v", err)
	}

	t.Run("Should accept a conforming payload", func(t *testing.T) {
		t.Parallel()

		verdict, err := registry.Validate(
			ctx,
			contract.Digest,
			json.RawMessage(`{"findings":[{"severity":"high","note":"fix"}]}`),
		)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if !verdict.Valid || len(verdict.Issues) != 0 || verdict.Unwrapped {
			t.Fatalf("Validate() verdict = %#v, want direct valid", verdict)
		}
	})

	t.Run("Should report the missing required field at its instance path", func(t *testing.T) {
		t.Parallel()

		verdict, err := registry.Validate(ctx, contract.Digest, json.RawMessage(`{"findings":[{"note":"fix"}]}`))
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if verdict.Valid || len(verdict.Issues) == 0 {
			t.Fatalf("Validate() verdict = %#v, want invalid issue", verdict)
		}
		if got, want := verdict.Issues[0].Path, "$.findings[0].severity"; got != want {
			t.Fatalf("Issue path = %q, want %q", got, want)
		}
		if strings.TrimSpace(verdict.Issues[0].Message) == "" {
			t.Fatal("Issue message is blank, want validator detail")
		}
	})

	t.Run("Should unwrap exactly one single-key object wrapper", func(t *testing.T) {
		t.Parallel()

		verdict, err := registry.Validate(ctx, contract.Digest, json.RawMessage(`{"output":{"findings":[]}}`))
		if err != nil {
			t.Fatalf("Validate(single wrapper) error = %v", err)
		}
		if !verdict.Valid || !verdict.Unwrapped {
			t.Fatalf("Validate(single wrapper) = %#v, want valid unwrapped", verdict)
		}
		double, err := registry.Validate(ctx, contract.Digest, json.RawMessage(`{"output":{"result":{"findings":[]}}}`))
		if err != nil {
			t.Fatalf("Validate(double wrapper) error = %v", err)
		}
		if double.Valid || double.Unwrapped {
			t.Fatalf("Validate(double wrapper) = %#v, want invalid", double)
		}
	})

	t.Run("Should sanitize raw bytes before validation and validator issues", func(t *testing.T) {
		t.Parallel()

		secretContract, pinErr := registry.Pin(ctx, json.RawMessage(`{
			"type":"object",
			"properties":{"note":{"type":"string","enum":["COMPOZY_CLAIM_secret-value"]}},
			"required":["note"]
		}`))
		if pinErr != nil {
			t.Fatalf("Pin(secret enum) error = %v", pinErr)
		}
		verdict, validateErr := registry.Validate(
			ctx,
			secretContract.Digest,
			json.RawMessage(`{"note":"COMPOZY_CLAIM_secret-value"}`),
		)
		if validateErr != nil {
			t.Fatalf("Validate(secret enum) error = %v", validateErr)
		}
		if verdict.Valid || len(verdict.Issues) == 0 {
			t.Fatalf("Validate(secret enum) verdict = %#v, want sanitized invalid", verdict)
		}
		for _, issue := range verdict.Issues {
			if strings.Contains(issue.Message, "secret-value") {
				t.Fatalf("validator issue leaked raw secret: %#v", issue)
			}
		}
	})
}

func TestRegistryCompiledCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemoryRegistryStore()
	registryValue := NewRegistry(store)
	registry := registryValue.(*registry)
	compileCounts := sync.Map{}
	registry.onCompile = func(digest string) {
		value, _ := compileCounts.LoadOrStore(digest, &atomic.Int64{})
		value.(*atomic.Int64).Add(1)
	}
	first, err := registry.Pin(ctx, json.RawMessage(`{"name":""}`))
	if err != nil {
		t.Fatalf("Pin(first) error = %v", err)
	}
	second, err := registry.Pin(ctx, json.RawMessage(`{"count":0}`))
	if err != nil {
		t.Fatalf("Pin(second) error = %v", err)
	}

	t.Run("Should compile one cache entry exactly once under concurrency", func(t *testing.T) {
		var wait sync.WaitGroup
		errorsByWorker := make(chan error, 100)
		for range 100 {
			wait.Go(func() {
				_, validateErr := registry.Validate(ctx, first.Digest, json.RawMessage(`{"name":"ok"}`))
				errorsByWorker <- validateErr
			})
		}
		wait.Wait()
		close(errorsByWorker)
		for validateErr := range errorsByWorker {
			if validateErr != nil {
				t.Fatalf("Validate() error = %v", validateErr)
			}
		}
		value, ok := compileCounts.Load(first.Digest)
		if !ok {
			t.Fatal("first contract was not compiled")
		}
		if got := value.(*atomic.Int64).Load(); got != 1 {
			t.Fatalf("first compile count = %d, want 1", got)
		}
		if _, err := registry.Validate(ctx, second.Digest, json.RawMessage(`{"count":1}`)); err != nil {
			t.Fatalf("Validate(second) error = %v", err)
		}
		value, ok = compileCounts.Load(second.Digest)
		if !ok {
			t.Fatal("second contract was not compiled")
		}
		if got := value.(*atomic.Int64).Load(); got != 1 {
			t.Fatalf("second compile count = %d, want 1", got)
		}
	})

	t.Run("Should evict the oldest compiled schema at the cache bound", func(t *testing.T) {
		for index := range compiledSchemaCacheLimit {
			contract, pinErr := registry.Pin(ctx, json.RawMessage(fmt.Sprintf(`{"field_%d":""}`, index)))
			if pinErr != nil {
				t.Fatalf("Pin(%d) error = %v", index, pinErr)
			}
			if _, validateErr := registry.Validate(
				ctx,
				contract.Digest,
				json.RawMessage(fmt.Sprintf(`{"field_%d":"ok"}`, index)),
			); validateErr != nil {
				t.Fatalf("Validate(%d) error = %v", index, validateErr)
			}
		}
		registry.mu.Lock()
		cacheSize := len(registry.cache)
		_, firstRetained := registry.cache[first.Digest]
		registry.mu.Unlock()
		if cacheSize != compiledSchemaCacheLimit || firstRetained {
			t.Fatalf("compiled cache size=%d first retained=%t, want bounded FIFO eviction", cacheSize, firstRetained)
		}
	})
}

func TestContractNormalization(t *testing.T) {
	t.Parallel()

	store := newMemoryRegistryStore()
	registry := NewRegistry(store)
	ctx := context.Background()

	t.Run("Should normalize shorthand and its expanded schema to one digest", func(t *testing.T) {
		t.Parallel()

		shorthand := json.RawMessage(`{"findings":[{"file":"","line":0,"severity":"","note":""}],"verdict":""}`)
		expanded := json.RawMessage(`{"type":"object","required":["findings","verdict"],"properties":{
			"findings":{"type":"array","items":{"type":"object",
				"required":["file","line","severity","note"],
				"properties":{"file":{"type":"string"},"line":{"type":"number"},
					"severity":{"type":"string"},"note":{"type":"string"}}}},
			"verdict":{"type":"string"}}}`)
		first, err := registry.Pin(ctx, shorthand)
		if err != nil {
			t.Fatalf("Pin(shorthand) error = %v", err)
		}
		second, err := registry.Pin(ctx, expanded)
		if err != nil {
			t.Fatalf("Pin(expanded) error = %v", err)
		}
		if first.Digest != second.Digest {
			t.Fatalf("shorthand digest = %q, expanded = %q", first.Digest, second.Digest)
		}
	})

	t.Run("Should classify a non-form as call_expect_invalid", func(t *testing.T) {
		t.Parallel()

		_, err := registry.Pin(ctx, json.RawMessage(`"string"`))
		if !IsCode(err, CodeExpectInvalid) {
			t.Fatalf("Pin(non-form) error = %v, want %s", err, CodeExpectInvalid)
		}
	})

	t.Run("Should distinguish shorthand fields from complete schema markers", func(t *testing.T) {
		t.Parallel()

		shorthand, err := NormalizeSchema(json.RawMessage(`{"type":"string"}`))
		if err != nil {
			t.Fatalf("NormalizeSchema(shorthand type field) error = %v", err)
		}
		if !strings.Contains(string(shorthand), `"properties":{"type":{"type":"string"}}`) {
			t.Fatalf("NormalizeSchema(shorthand type field) = %s", shorthand)
		}

		for _, schema := range []json.RawMessage{
			json.RawMessage(`{"$comment":"authored schema"}`),
			json.RawMessage(`{"uniqueItems":true}`),
		} {
			canonical, normalizeErr := NormalizeSchema(schema)
			if normalizeErr != nil {
				t.Fatalf("NormalizeSchema(%s) error = %v", schema, normalizeErr)
			}
			if strings.Contains(string(canonical), `"properties"`) {
				t.Fatalf("NormalizeSchema(%s) = %s, want complete schema", schema, canonical)
			}
		}
	})
}

func TestRepairPromptAndContractFaults(t *testing.T) {
	t.Parallel()

	t.Run("Should render ten deterministic issues without a schema", func(t *testing.T) {
		t.Parallel()

		issues := make([]ValidationIssue, 25)
		for index := range issues {
			issues[index] = ValidationIssue{Path: "$.field", Message: "issue-" + string(rune('a'+index))}
		}
		prompt := BuildRepairPrompt(issues)
		if got := strings.Count(prompt, "\n- "); got != 10 {
			t.Fatalf("repair issue count = %d, want 10", got)
		}
		if !strings.Contains(prompt, "(+15 more)") {
			t.Fatalf("Repair prompt = %q, want remainder", prompt)
		}
		if strings.Contains(prompt, "\"properties\"") || strings.Contains(prompt, "output_schema") {
			t.Fatalf("Repair prompt re-pasted schema: %q", prompt)
		}
		if prompt != BuildRepairPrompt(issues) {
			t.Fatal("BuildRepairPrompt() is not deterministic")
		}
	})

	t.Run("Should classify a corrupt stored schema as a contract fault", func(t *testing.T) {
		t.Parallel()

		store := newMemoryRegistryStore()
		store.contracts["sha256:corrupt"] = Contract{
			Digest: "sha256:corrupt",
			Schema: json.RawMessage(`{"type":"not-a-type"}`),
		}
		_, err := NewRegistry(store).Validate(context.Background(), "sha256:corrupt", json.RawMessage(`{}`))
		if !IsCode(err, CodeContractCompile) || !IsContractFault(err) {
			t.Fatalf("Validate(corrupt) error = %#v, want contract compile fault", err)
		}
	})
}
