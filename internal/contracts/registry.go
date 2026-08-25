package contracts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type compiledEntry struct {
	once     sync.Once
	schema   *jsonschema.Schema
	err      error
	compiles atomic.Int64
}

type registry struct {
	store RegistryStore
	mu    sync.Mutex
	cache map[string]*compiledEntry
}

// NewRegistry creates a registry over the caller-owned immutable store.
func NewRegistry(store RegistryStore) Registry {
	return &registry{store: store, cache: make(map[string]*compiledEntry)}
}

func (r *registry) Pin(ctx context.Context, raw json.RawMessage) (Contract, error) {
	canonical, err := normalizeSchema(raw)
	if err != nil {
		return Contract{}, newError(CodeExpectInvalid, FaultContract, err.Error(), err)
	}
	if err := validateSecretFieldAuthorship(canonical); err != nil {
		return Contract{}, newError(CodeExpectInvalid, FaultContract, err.Error(), err)
	}
	digestBytes := sha256.Sum256(canonical)
	contract := Contract{
		Digest: "sha256:" + hex.EncodeToString(digestBytes[:]),
		Schema: cloneRaw(canonical),
	}
	if _, err := compileSchema(contract); err != nil {
		return Contract{}, newError(CodeExpectInvalid, FaultContract, err.Error(), err)
	}
	if r.store == nil {
		return Contract{}, newError(CodeExpectInvalid, FaultContract, "registry store is required", nil)
	}
	if err := r.store.PutContract(ctx, contract); err != nil {
		return Contract{}, fmt.Errorf("pin contract %s: %w", contract.Digest, err)
	}
	return contract, nil
}

func (r *registry) Resolve(ctx context.Context, digest string) (Contract, error) {
	if r.store == nil {
		return Contract{}, newError(CodeContractNotFound, FaultContract, "registry store is required", nil)
	}
	contract, err := r.store.GetContract(ctx, strings.TrimSpace(digest))
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			return Contract{}, newError(
				CodeContractNotFound,
				FaultContract,
				fmt.Sprintf("contract %q was not found", digest),
				err,
			)
		}
		return Contract{}, fmt.Errorf("resolve contract %q: %w", digest, err)
	}
	if contract.Digest == "" || len(contract.Schema) == 0 {
		cause := errors.New("stored contract row is incomplete")
		return Contract{}, newError(CodeContractCompile, FaultContract, cause.Error(), cause)
	}
	contract.Schema = cloneRaw(contract.Schema)
	return contract, nil
}

func (r *registry) Validate(ctx context.Context, digest string, payload json.RawMessage) (Verdict, error) {
	contract, err := r.Resolve(ctx, digest)
	if err != nil {
		return Verdict{}, err
	}
	compiled, err := r.compiled(contract)
	if err != nil {
		return Verdict{}, newError(CodeContractCompile, FaultContract, err.Error(), err)
	}
	return validatePayload(compiled, payload), nil
}

func (r *registry) compiled(contract Contract) (*jsonschema.Schema, error) {
	r.mu.Lock()
	entry := r.cache[contract.Digest]
	if entry == nil {
		entry = &compiledEntry{}
		r.cache[contract.Digest] = entry
	}
	r.mu.Unlock()
	entry.once.Do(func() {
		entry.compiles.Add(1)
		entry.schema, entry.err = compileSchema(contract)
	})
	return entry.schema, entry.err
}

func compileSchema(contract Contract) (*jsonschema.Schema, error) {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(contract.Schema))
	if err != nil {
		return nil, fmt.Errorf("parse contract schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	resource := strings.TrimSpace(contract.Digest)
	if resource == "" {
		resource = "contract.json"
	}
	if err := compiler.AddResource(resource, value); err != nil {
		return nil, fmt.Errorf("add contract schema: %w", err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("compile contract schema: %w", err)
	}
	return compiled, nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}
