package llm

import "github.com/compozy/compozy/engine/llm/contracts"

type MemoryProvider = contracts.MemoryProvider

type Memory = contracts.Memory

type MessageRole = contracts.MessageRole

const (
	MessageRoleUser      MessageRole = contracts.MessageRoleUser
	MessageRoleAssistant MessageRole = contracts.MessageRoleAssistant
	MessageRoleSystem    MessageRole = contracts.MessageRoleSystem
	MessageRoleTool      MessageRole = contracts.MessageRoleTool
)

type Message = contracts.Message
