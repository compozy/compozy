package core

import (
	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
)

func agentMemoryDocumentPayload(doc agentmemory.Document) contract.AgentMemoryDocument {
	return contract.AgentMemoryDocument{Content: doc.Content, Digest: doc.Digest, Budget: contract.AgentMemoryBudget{
		Lines: doc.Budget.Lines, MaxLines: doc.Budget.MaxLines, Bytes: doc.Budget.Bytes, MaxBytes: doc.Budget.MaxBytes,
	}}
}
func agentMemoryHistoryPayload(revisions []agentmemory.Revision) contract.AgentMemoryHistoryResponse {
	result := contract.AgentMemoryHistoryResponse{Revisions: make([]contract.AgentMemoryRevision, 0, len(revisions))}
	for _, revision := range revisions {
		result.Revisions = append(result.Revisions, contract.AgentMemoryRevision{ID: revision.ID, Digest: revision.Digest})
	}
	return result
}
func agentMemorySearchPayload(results []agentmemory.SearchResult) contract.AgentMemorySearchResponse {
	response := contract.AgentMemorySearchResponse{Results: make([]contract.AgentMemorySearchResult, 0, len(results))}
	for _, result := range results {
		response.Results = append(response.Results, contract.AgentMemorySearchResult{SessionID: result.SessionID, Source: result.Source, Text: result.Text, File: result.File, Line: result.Line, Score: result.Score})
	}
	return response
}
