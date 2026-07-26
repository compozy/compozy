package contract

import (
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const HostAPIMethodClarifyAsk = extensionprotocol.HostAPIMethodClarifyAsk

// ClarifyAskParams carries daemon-issued invocation authority and extension-authored question data.
type ClarifyAskParams struct {
	InvocationID string   `json:"invocation_id"`
	Question     string   `json:"question"`
	Choices      []string `json:"choices,omitempty"`
}

func clarifyHostAPIMethodSpec() HostAPIMethodSpec {
	return HostAPIMethodSpec{
		Method: HostAPIMethodClarifyAsk,
		Params: NamedType{Name: "ClarifyAskParams", Value: ClarifyAskParams{}},
		Result: NamedType{Name: "ClarifyAnswer", Value: toolspkg.ClarifyAnswer{}},
	}
}
