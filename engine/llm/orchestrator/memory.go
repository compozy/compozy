package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
)

const memoryStoreTimeout = 30 * time.Second // TODO: source from config.FromContext(ctx)

type MemoryContext struct {
	memories   map[string]contracts.Memory
	references []core.MemoryReference
}

// References returns a copy of the configured memory references for snapshotting.
func (m *MemoryContext) References() []core.MemoryReference {
	if m == nil || len(m.references) == 0 {
		return nil
	}
	out := make([]core.MemoryReference, len(m.references))
	copy(out, m.references)
	return out
}

// FailureEpisode captures minimal context about a failed orchestration attempt.
type FailureEpisode struct {
	PlanSummary  string
	ErrorSummary string
}

type MemoryManager interface {
	Prepare(ctx context.Context, request Request) *MemoryContext
	Inject(ctx context.Context, base []llmadapter.Message, ctxData *MemoryContext) []llmadapter.Message
	StoreAsync(
		ctx context.Context,
		ctxData *MemoryContext,
		response *llmadapter.LLMResponse,
		messages []llmadapter.Message,
		request Request,
	)
	StoreFailureEpisode(ctx context.Context, ctxData *MemoryContext, request Request, episode FailureEpisode)
	Compact(ctx context.Context, loopCtx *LoopContext, usage telemetry.ContextUsage) error
}

type memoryManager struct {
	provider contracts.MemoryProvider
	sync     MemorySync
	hook     AsyncHook
}

func NewMemoryManager(provider contracts.MemoryProvider, sync MemorySync, hook AsyncHook) MemoryManager {
	return &memoryManager{provider: provider, sync: sync, hook: hook}
}

//nolint:gocritic // Request is copied to avoid accidental mutation while materializing memory contexts.
func (m *memoryManager) Prepare(ctx context.Context, request Request) *MemoryContext {
	log := logger.FromContext(ctx)
	if m.provider == nil {
		log.Debug("No memory provider available")
		return nil
	}
	if request.Agent == nil {
		log.Debug("No agent on request; skipping memory preparation")
		return nil
	}
	references := request.Agent.Memory
	if len(references) == 0 {
		log.Debug("No memory references configured for agent", "agent_id", request.Agent.ID)
		return nil
	}
	memories := make(map[string]contracts.Memory)
	for _, ref := range references {
		memory, err := m.provider.GetMemory(ctx, ref.ID, ref.Key)
		if err != nil {
			log.Error("Failed to get memory instance", "memory_id", ref.ID, "error", core.RedactError(err))
			continue
		}
		if memory == nil {
			log.Warn("Memory instance is nil", "memory_id", ref.ID)
			continue
		}
		log.Debug("Memory instance retrieved successfully", "memory_id", ref.ID, "instance_id", memory.GetID())
		memories[ref.ID] = memory
	}
	if len(memories) == 0 {
		return nil
	}
	return &MemoryContext{memories: memories, references: references}
}

func (m *memoryManager) Inject(
	ctx context.Context,
	base []llmadapter.Message,
	ctxData *MemoryContext,
) []llmadapter.Message {
	if ctxData == nil || len(ctxData.memories) == 0 {
		return base
	}
	log := logger.FromContext(ctx)
	var memoryMessages []llmadapter.Message
	appendFromMemory := func(id string) {
		memory := ctxData.memories[id]
		if memory == nil {
			return
		}
		msgs, err := memory.Read(ctx)
		if err != nil {
			log.Warn("Failed to read messages from memory", "memory_id", id, "error", core.RedactError(err))
			return
		}
		for _, msg := range msgs {
			memoryMessages = append(memoryMessages, llmadapter.Message{Role: string(msg.Role), Content: msg.Content})
		}
	}
	if len(ctxData.references) > 0 {
		for _, ref := range ctxData.references {
			appendFromMemory(ref.ID)
		}
	} else {
		ids := make([]string, 0, len(ctxData.memories))
		for id := range ctxData.memories {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			appendFromMemory(id)
		}
	}
	if len(memoryMessages) == 0 {
		return base
	}
	combined := make([]llmadapter.Message, 0, len(memoryMessages)+len(base))
	combined = append(combined, memoryMessages...)
	combined = append(combined, base...)
	return combined
}

//nolint:gocritic // Request copied to preserve snapshot consistency when storing asynchronously.
func (m *memoryManager) StoreAsync(
	ctx context.Context,
	ctxData *MemoryContext,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
	request Request,
) {
	if !m.shouldStoreAsync(ctxData, response, messages) {
		return
	}
	userMsg, found := m.findLastUserMessage(messages)
	if !found {
		return
	}
	go m.storeInBackground(ctx, ctxData, response, &userMsg, request)
}

//nolint:gocritic // Request copied to preserve snapshot consistency when storing asynchronously.
func (m *memoryManager) StoreFailureEpisode(
	ctx context.Context,
	ctxData *MemoryContext,
	request Request,
	episode FailureEpisode,
) {
	if !m.shouldStoreFailureEpisode(ctxData, episode) {
		return
	}
	messages := buildEpisodeMessages(episode)
	if len(messages) == 0 {
		return
	}
	go m.storeFailureEpisodeInBackground(ctx, ctxData, request, messages)
}

func (m *memoryManager) shouldStoreAsync(
	ctxData *MemoryContext,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
) bool {
	if ctxData == nil || len(ctxData.memories) == 0 {
		return false
	}
	if response == nil || response.Content == "" {
		return false
	}
	return len(messages) > 0
}

func (m *memoryManager) shouldStoreFailureEpisode(ctxData *MemoryContext, episode FailureEpisode) bool {
	if ctxData == nil || len(ctxData.memories) == 0 {
		return false
	}
	if strings.TrimSpace(episode.PlanSummary) == "" && strings.TrimSpace(episode.ErrorSummary) == "" {
		return false
	}
	return true
}

func (m *memoryManager) findLastUserMessage(messages []llmadapter.Message) (llmadapter.Message, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llmadapter.RoleUser {
			return messages[i], true
		}
	}
	return llmadapter.Message{}, false
}

//nolint:gocritic // Request captured by value to avoid races while background goroutine persists state.
func (m *memoryManager) storeInBackground(
	ctx context.Context,
	ctxData *MemoryContext,
	response *llmadapter.LLMResponse,
	lastUser *llmadapter.Message,
	request Request,
) {
	log := logger.FromContext(ctx)
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryStoreTimeout)
	defer cancel()
	memoryIDs := m.extractMemoryIDs(ctxData)
	assistantMsg := llmadapter.Message{Role: llmadapter.RoleAssistant, Content: response.Content}
	err := m.executeStoreWithLocks(bgCtx, ctxData, &assistantMsg, lastUser, memoryIDs)
	if err != nil {
		m.logStoreError(log, err, request)
	}
	if m.hook != nil {
		m.hook.OnMemoryStoreComplete(err)
	}
}

//nolint:gocritic // Request captured by value to avoid races while background goroutine persists state.
func (m *memoryManager) storeFailureEpisodeInBackground(
	ctx context.Context,
	ctxData *MemoryContext,
	request Request,
	messages []contracts.Message,
) {
	log := logger.FromContext(ctx)
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryStoreTimeout)
	defer cancel()
	memoryIDs := m.extractMemoryIDs(ctxData)
	err := m.executeEpisodeStoreWithLocks(bgCtx, ctxData, messages, memoryIDs)
	if err != nil {
		m.logFailureEpisodeError(log, err, request)
	}
	if m.hook != nil {
		m.hook.OnMemoryStoreComplete(err)
	}
}

func (m *memoryManager) extractMemoryIDs(ctxData *MemoryContext) []string {
	memoryIDs := make([]string, 0, len(ctxData.references))
	for _, ref := range ctxData.references {
		if ref.Mode == core.MemoryModeReadOnly {
			continue
		}
		if mem := ctxData.memories[ref.ID]; mem != nil {
			memoryIDs = append(memoryIDs, mem.GetID())
		}
	}
	return memoryIDs
}

func (m *memoryManager) executeStoreWithLocks(
	bgCtx context.Context,
	ctxData *MemoryContext,
	assistantMsg *llmadapter.Message,
	lastUser *llmadapter.Message,
	memoryIDs []string,
) error {
	storeFn := func() error {
		return m.store(bgCtx, ctxData.memories, ctxData.references, assistantMsg, lastUser)
	}
	var err error
	if m.sync != nil {
		m.sync.WithMultipleLocks(memoryIDs, func() { err = storeFn() })
	} else {
		err = storeFn()
	}
	return err
}

func (m *memoryManager) executeEpisodeStoreWithLocks(
	bgCtx context.Context,
	ctxData *MemoryContext,
	messages []contracts.Message,
	memoryIDs []string,
) error {
	if len(messages) == 0 {
		return nil
	}
	storeFn := func() error {
		return m.storeMessages(bgCtx, ctxData.memories, ctxData.references, messages)
	}
	var err error
	if m.sync != nil {
		m.sync.WithMultipleLocks(memoryIDs, func() { err = storeFn() })
	} else {
		err = storeFn()
	}
	return err
}

//nolint:gocritic // Request data copied for logging to avoid mutation while formatting metadata.
func (m *memoryManager) logStoreError(log logger.Logger, err error, request Request) {
	agentID := ""
	if request.Agent != nil {
		agentID = request.Agent.ID
	}
	actionID := ""
	if request.Action != nil {
		actionID = request.Action.ID
	}
	log.Error(
		"Failed to store response in memory",
		"error",
		core.RedactError(err),
		"agent_id",
		agentID,
		"action_id",
		actionID,
	)
}

//nolint:gocritic // Request copied for safe structured logging.
func (m *memoryManager) logFailureEpisodeError(log logger.Logger, err error, request Request) {
	if err == nil {
		return
	}
	agentID := ""
	if request.Agent != nil {
		agentID = request.Agent.ID
	}
	actionID := ""
	if request.Action != nil {
		actionID = request.Action.ID
	}
	log.Error(
		"Failed to store failure episode in memory",
		"error",
		core.RedactError(err),
		"agent_id",
		agentID,
		"action_id",
		actionID,
	)
}

func (m *memoryManager) store(
	ctx context.Context,
	memories map[string]contracts.Memory,
	refs []core.MemoryReference,
	assistant *llmadapter.Message,
	user *llmadapter.Message,
) error {
	if assistant == nil || user == nil {
		return fmt.Errorf("memory store: nil message pointer(s): assistant=%v user=%v", assistant == nil, user == nil)
	}
	msgs := []contracts.Message{
		{
			Role:    contracts.MessageRole(user.Role),
			Content: user.Content,
		},
		{
			Role:    contracts.MessageRole(assistant.Role),
			Content: assistant.Content,
		},
	}
	return m.storeMessages(ctx, memories, refs, msgs)
}

func (m *memoryManager) storeMessages(
	ctx context.Context,
	memories map[string]contracts.Memory,
	refs []core.MemoryReference,
	msgs []contracts.Message,
) error {
	if len(memories) == 0 || len(msgs) == 0 {
		return nil
	}
	log := logger.FromContext(ctx)
	var errs []error
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("memory store canceled: %w", err)
		}
		memory, exists := memories[ref.ID]
		if !exists {
			log.Debug("Memory reference not found; skipping", "memory_id", ref.ID)
			continue
		}
		if ref.Mode == core.MemoryModeReadOnly {
			log.Debug("Skipping read-only memory", "memory_id", ref.ID)
			continue
		}
		if err := memory.AppendMany(ctx, msgs); err != nil {
			log.Error(
				"Failed to append messages to memory atomically",
				"memory_id",
				ref.ID,
				"error",
				core.RedactError(err),
			)
			err = fmt.Errorf("failed to append messages to memory %s: %w", ref.ID, err)
			errs = append(errs, err)
			continue
		}
		log.Debug("Messages stored atomically in memory", "memory_id", ref.ID)
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory storage errors: %w", errors.Join(errs...))
	}
	return nil
}

func buildEpisodeMessages(episode FailureEpisode) []contracts.Message {
	messages := make([]contracts.Message, 0, 2)
	if text := strings.TrimSpace(episode.PlanSummary); text != "" {
		messages = append(messages, contracts.Message{
			Role:    contracts.MessageRoleAssistant,
			Content: text,
		})
	}
	if text := strings.TrimSpace(episode.ErrorSummary); text != "" {
		messages = append(messages, contracts.Message{
			Role:    contracts.MessageRoleSystem,
			Content: text,
		})
	}
	return messages
}
