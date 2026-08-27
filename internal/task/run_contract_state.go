package task

import "github.com/compozy/compozy/internal/contracts"

// RunContractState keeps per-run idempotency and structured-result policy off the hot Run value.
// Embedding preserves the flat durable JSON contract.
type RunContractState struct {
	IdempotencyKey string                `json:"idempotency_key,omitempty"`
	ExpectDigest   string                `json:"expect_digest,omitempty"`
	ResultBudget   *contracts.ByteBudget `json:"result_budget,omitempty"`
}

// ContractStateValue returns the materialized contract state or its zero value.
func (r Run) ContractStateValue() RunContractState {
	if r.RunContractState == nil {
		return RunContractState{}
	}
	return *r.RunContractState
}

// SetContractState materializes the contract state for safe promoted-field access.
func (r *Run) SetContractState(state RunContractState) {
	if r == nil {
		return
	}
	r.RunContractState = &state
}

// IdempotencyKeyValue returns the durable idempotency key without requiring materialized state.
func (r Run) IdempotencyKeyValue() string {
	return r.ContractStateValue().IdempotencyKey
}

// ExpectDigestValue returns the result-contract digest without requiring materialized state.
func (r Run) ExpectDigestValue() string {
	return r.ContractStateValue().ExpectDigest
}

// ResultBudgetValue returns the result byte budget without requiring materialized state.
func (r Run) ResultBudgetValue() *contracts.ByteBudget {
	return r.ContractStateValue().ResultBudget
}
