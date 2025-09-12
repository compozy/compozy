package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/tokens"
)

// MessageSummarizer defines an interface for summarizing a list of messages.
type MessageSummarizer interface {
	SummarizeMessages(
		ctx context.Context,
		messages []memcore.MessageWithTokens,
		targetSummaryTokenCount int,
	) (llm.Message, []memcore.MessageWithTokens, []memcore.MessageWithTokens, error)
}

// RuleBasedSummarizer implements deterministic summarization based on rules.
// For v1, it combines the first message and N most recent messages.
type RuleBasedSummarizer struct {
	tokenCounter memcore.TokenCounter
	// Configuration for how many recent messages to keep, etc.
	// For example:
	KeepFirstNMessages int // Number of initial messages to always keep (e.g. system prompts)
	KeepLastNMessages  int // Number of recent messages to keep for continuity
	// Fallback token estimation ratio when token counting fails
	// Lower values are more conservative for non-English content
	TokenFallbackRatio int // Characters per token ratio (default: 3)
}

// NewRuleBasedSummarizer creates a new summarizer.
func NewRuleBasedSummarizer(counter memcore.TokenCounter, keepFirstN, keepLastN int) *RuleBasedSummarizer {
	if keepFirstN < 0 {
		keepFirstN = 0
	}
	if keepLastN < 0 {
		keepLastN = 1 // Keep at least one recent message if summarization occurs
	}
	return &RuleBasedSummarizer{
		tokenCounter:       counter,
		KeepFirstNMessages: keepFirstN,
		KeepLastNMessages:  keepLastN,
		TokenFallbackRatio: 3, // Conservative default for better cross-language support
	}
}

// NewRuleBasedSummarizerWithOptions creates a new summarizer with custom token fallback ratio.
func NewRuleBasedSummarizerWithOptions(
	counter memcore.TokenCounter,
	keepFirstN, keepLastN, fallbackRatio int,
) *RuleBasedSummarizer {
	if keepFirstN < 0 {
		keepFirstN = 0
	}
	if keepLastN < 0 {
		keepLastN = 1 // Keep at least one recent message if summarization occurs
	}
	if fallbackRatio <= 0 {
		fallbackRatio = 3 // Conservative default
	}
	return &RuleBasedSummarizer{
		tokenCounter:       counter,
		KeepFirstNMessages: keepFirstN,
		KeepLastNMessages:  keepLastN,
		TokenFallbackRatio: fallbackRatio,
	}
}

// SummarizeMessages generates a summary message and identifies messages that were summarized.
// Returns: the summary message, and the list of messages that were *not* part
// of the summary (i.e., those to be removed).
func (rbs *RuleBasedSummarizer) SummarizeMessages(
	ctx context.Context,
	messagesToProcess []memcore.MessageWithTokens,
	targetSummaryTokenCount int,
) (summary llm.Message, remainingMessages []memcore.MessageWithTokens,
	summarizedOriginalMessages []memcore.MessageWithTokens, err error) {
	if !rbs.shouldSummarize(messagesToProcess) {
		return llm.Message{}, messagesToProcess, nil, nil
	}
	return rbs.performSummarization(ctx, messagesToProcess, targetSummaryTokenCount)
}

func (rbs *RuleBasedSummarizer) performSummarization(
	ctx context.Context,
	messagesToProcess []memcore.MessageWithTokens,
	targetSummaryTokenCount int,
) (llm.Message, []memcore.MessageWithTokens, []memcore.MessageWithTokens, error) {
	keptMessages, messagesThatFormedSummaryText := rbs.categorizeMessages(messagesToProcess)
	if len(messagesThatFormedSummaryText) == 0 {
		return llm.Message{}, messagesToProcess, nil, nil
	}
	summaryContent, err := rbs.buildSummaryContent(ctx, messagesThatFormedSummaryText, targetSummaryTokenCount)
	if err != nil {
		return llm.Message{}, messagesToProcess, nil, err
	}
	finalSummaryMessage := llm.Message{
		Role:    "system",
		Content: summaryContent,
	}
	return finalSummaryMessage, keptMessages, messagesThatFormedSummaryText, nil
}

func (rbs *RuleBasedSummarizer) shouldSummarize(messagesToProcess []memcore.MessageWithTokens) bool {
	return len(messagesToProcess) > rbs.KeepFirstNMessages+rbs.KeepLastNMessages
}

func (rbs *RuleBasedSummarizer) categorizeMessages(
	messagesToProcess []memcore.MessageWithTokens,
) ([]memcore.MessageWithTokens, []memcore.MessageWithTokens) {
	keptMessages := make([]memcore.MessageWithTokens, 0, rbs.KeepFirstNMessages+rbs.KeepLastNMessages)
	if rbs.KeepFirstNMessages > 0 && len(messagesToProcess) >= rbs.KeepFirstNMessages {
		keptMessages = append(keptMessages, messagesToProcess[:rbs.KeepFirstNMessages]...)
	}
	firstIdxForSummary := rbs.KeepFirstNMessages
	lastIdxToKeep := len(messagesToProcess) - rbs.KeepLastNMessages
	if lastIdxToKeep <= firstIdxForSummary {
		return messagesToProcess, nil
	}
	messagesThatFormedSummaryText := messagesToProcess[firstIdxForSummary:lastIdxToKeep]
	if rbs.KeepLastNMessages > 0 {
		keptMessages = append(keptMessages, messagesToProcess[lastIdxToKeep:]...)
	}
	return keptMessages, messagesThatFormedSummaryText
}

func (rbs *RuleBasedSummarizer) buildSummaryContent(
	ctx context.Context,
	messages []memcore.MessageWithTokens,
	targetTokenCount int,
) (string, error) {
	baseSummary := rbs.buildBaseSummary(messages)
	if len(messages) <= 1 {
		return rbs.truncateSummaryIfNeeded(ctx, baseSummary, targetTokenCount), nil
	}
	enhancedSummary, err := rbs.addLastMessageToSummary(ctx, baseSummary, messages, targetTokenCount)
	if err != nil {
		return "", err
	}
	return rbs.truncateSummaryIfNeeded(ctx, enhancedSummary, targetTokenCount), nil
}

func (rbs *RuleBasedSummarizer) buildBaseSummary(messages []memcore.MessageWithTokens) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Summary of %d messages: ", len(messages)))
	if len(messages) > 0 {
		if msg, ok := messages[0].Message.(llm.Message); ok {
			builder.WriteString(fmt.Sprintf("[%s]: %s", msg.Role, msg.Content))
		}
	}
	return builder.String()
}

func (rbs *RuleBasedSummarizer) addLastMessageToSummary(
	ctx context.Context,
	baseSummary string,
	messages []memcore.MessageWithTokens,
	targetTokenCount int,
) (string, error) {
	var builder strings.Builder
	builder.WriteString(baseSummary)
	return rbs.appendLastMessage(ctx, &builder, messages, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) appendLastMessage(
	ctx context.Context,
	builder *strings.Builder,
	messages []memcore.MessageWithTokens,
	targetTokenCount int,
) (string, error) {
	builder.WriteString(" ... ")
	lastMessage := messages[len(messages)-1]
	var lastMsgContent string
	if msg, ok := lastMessage.Message.(llm.Message); ok {
		lastMsgContent = fmt.Sprintf("[%s]: %s", msg.Role, msg.Content)
	}
	if rbs.canFitLastMessage(ctx, builder.String(), lastMsgContent, targetTokenCount) {
		builder.WriteString(lastMsgContent)
	}
	return builder.String(), nil
}

func (rbs *RuleBasedSummarizer) canFitLastMessage(
	ctx context.Context,
	currentContent, lastMsgContent string,
	targetTokenCount int,
) bool {
	if targetTokenCount <= 0 {
		return true
	}
	currentTokens := rbs.getTokenCount(ctx, currentContent)
	lastTokens := rbs.getTokenCount(ctx, lastMsgContent)
	return currentTokens+lastTokens < targetTokenCount
}

func (rbs *RuleBasedSummarizer) getTokenCount(ctx context.Context, content string) int {
	tokens, err := rbs.tokenCounter.CountTokens(ctx, content)
	if err != nil {
		// Use configurable fallback ratio for better cross-language support
		// Most languages have higher character-to-token ratios than English
		fallbackRatio := rbs.TokenFallbackRatio
		if fallbackRatio <= 0 {
			fallbackRatio = 3 // Conservative default
		}
		return len(content) / fallbackRatio
	}
	return tokens
}

func (rbs *RuleBasedSummarizer) truncateSummaryIfNeeded(
	ctx context.Context,
	summaryStr string,
	targetTokenCount int,
) string {
	if targetTokenCount <= 0 {
		return summaryStr
	}
	summaryTokens := rbs.getTokenCount(ctx, summaryStr)
	if summaryTokens <= targetTokenCount {
		return summaryStr
	}
	return rbs.performTruncation(ctx, summaryStr, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) performTruncation(ctx context.Context, summaryStr string, targetTokenCount int) string {
	if tiktokenCounter, ok := rbs.tokenCounter.(*tokens.TiktokenCounter); ok {
		return rbs.truncateUsingTokenizer(ctx, summaryStr, targetTokenCount, tiktokenCounter)
	}
	return rbs.truncateUsingCharacterEstimate(summaryStr, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) truncateUsingTokenizer(
	ctx context.Context,
	summaryStr string,
	targetTokenCount int,
	tiktokenCounter *tokens.TiktokenCounter,
) string {
	base := context.WithoutCancel(ctx)
	tokenList, err := tiktokenCounter.EncodeTokens(base, summaryStr)
	if err != nil {
		// Fallback to character-based estimation if tokenization fails
		return rbs.truncateUsingCharacterEstimate(summaryStr, targetTokenCount)
	}
	if len(tokenList) > targetTokenCount {
		if targetTokenCount <= 0 {
			return ""
		}
		reserve := 3 // space for "..."
		if targetTokenCount <= reserve {
			reserve = 0
		}
		cutoff := targetTokenCount - reserve
		if cutoff < 0 {
			cutoff = 0
		}
		tokenList = tokenList[:cutoff]
		truncatedStr, err := tiktokenCounter.DecodeTokens(base, tokenList)
		if err != nil {
			// Fallback to character-based estimation if decoding fails
			return rbs.truncateUsingCharacterEstimate(summaryStr, targetTokenCount)
		}
		if reserve > 0 {
			return truncatedStr + "..."
		}
		return truncatedStr
	}
	return summaryStr
}

func (rbs *RuleBasedSummarizer) truncateUsingCharacterEstimate(summaryStr string, targetTokenCount int) string {
	targetChars := targetTokenCount * rbs.TokenFallbackRatio
	if targetChars <= 0 {
		return ""
	}
	if len(summaryStr) <= targetChars {
		return summaryStr
	}
	// If not enough room for ellipsis, return strict cut
	if targetChars <= 3 {
		if targetChars > len(summaryStr) {
			targetChars = len(summaryStr)
		}
		return summaryStr[:targetChars]
	}
	end := targetChars - 3
	if end < 0 {
		end = 0
	}
	if end > len(summaryStr) {
		end = len(summaryStr)
	}
	return summaryStr[:end] + "..."
}

// HybridFlushingStrategy applies a flushing strategy, potentially involving summarization.
type HybridFlushingStrategy struct {
	config       *memcore.FlushingStrategyConfig
	summarizer   MessageSummarizer   // e.g., RuleBasedSummarizer
	tokenManager *TokenMemoryManager // To get token counts and apply limits post-flush
}

// NewHybridFlushingStrategy creates a new hybrid flushing strategy.
func NewHybridFlushingStrategy(
	config *memcore.FlushingStrategyConfig,
	summarizer MessageSummarizer,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	if config == nil {
		return nil, fmt.Errorf("flushing strategy config cannot be nil")
	}
	if summarizer == nil && config.Type == memcore.HybridSummaryFlushing {
		return nil, fmt.Errorf("summarizer cannot be nil for hybrid_summary flushing type")
	}
	if tokenManager == nil {
		return nil, fmt.Errorf("token manager cannot be nil")
	}
	return &HybridFlushingStrategy{
		config:       config,
		summarizer:   summarizer,
		tokenManager: tokenManager,
	}, nil
}

// ShouldFlush implements the instance.FlushStrategy interface.
// Checks if flushing is needed based on current state and config.
func (hfs *HybridFlushingStrategy) ShouldFlush(tokenCount, _ int, config *memcore.Resource) bool {
	if hfs.config == nil {
		return false
	}

	// Determine effective max tokens for threshold calculation
	effectiveMaxTokens := config.MaxTokens
	if effectiveMaxTokens == 0 && config.MaxContextRatio > 0 {
		modelContextSize := config.ModelContextSize
		if modelContextSize == 0 {
			modelContextSize = 4096 // Default fallback
		}
		effectiveMaxTokens = int(float64(modelContextSize) * config.MaxContextRatio)
	}

	if effectiveMaxTokens > 0 && hfs.config.SummarizeThreshold > 0 {
		thresholdTokens := int(float64(effectiveMaxTokens) * hfs.config.SummarizeThreshold)
		if tokenCount >= thresholdTokens {
			return true
		}
	}

	return false
}

// ShouldFlushByCount checks if flushing is needed based on message and token counts.
// This is an optimized version that doesn't require allocating message arrays.
func (hfs *HybridFlushingStrategy) ShouldFlushByCount(
	_ context.Context,
	messageCount int, //nolint:revive // Reserved for future message count based thresholds
	currentTotalTokens int,
) bool {
	if hfs.config == nil {
		return false
	}

	// Determine effective max tokens for threshold calculation
	effectiveMaxTokens := hfs.tokenManager.config.MaxTokens
	if effectiveMaxTokens == 0 && hfs.tokenManager.config.MaxContextRatio > 0 {
		modelContextSize := hfs.tokenManager.config.ModelContextSize
		if modelContextSize == 0 {
			modelContextSize = 4096 // Default fallback
		}
		effectiveMaxTokens = int(float64(modelContextSize) * hfs.tokenManager.config.MaxContextRatio)
	}

	if effectiveMaxTokens > 0 && hfs.config.SummarizeThreshold > 0 {
		thresholdTokens := int(float64(effectiveMaxTokens) * hfs.config.SummarizeThreshold)
		if currentTotalTokens >= thresholdTokens {
			return true
		}
	}

	// Could add message count based threshold too if needed
	// The messageCount parameter is available here for future use
	return false
}

// FlushMessages applies the flushing strategy to the given messages.
// Returns the new set of messages (potentially with a summary message replacing some old ones)
// and the new total token count.
func (hfs *HybridFlushingStrategy) FlushMessages(
	ctx context.Context,
	currentMessages []memcore.MessageWithTokens, // Messages with pre-calculated token counts
) (newMessages []memcore.MessageWithTokens, newTotalTokens int, summaryGenerated bool, err error) {
	switch hfs.config.Type {
	case memcore.HybridSummaryFlushing:
		return hfs.handleHybridSummaryFlushing(ctx, currentMessages)
	case memcore.SimpleFIFOFlushing:
		return hfs.handleSimpleFIFOFlushing(currentMessages)
	default:
		return hfs.handleUnsupportedFlushingType(currentMessages)
	}
}

// handleHybridSummaryFlushing handles hybrid summary flushing strategy
func (hfs *HybridFlushingStrategy) handleHybridSummaryFlushing(
	ctx context.Context,
	currentMessages []memcore.MessageWithTokens,
) ([]memcore.MessageWithTokens, int, bool, error) {
	if hfs.summarizer == nil {
		return currentMessages, calculateTotalTokens(currentMessages), false, nil
	}
	targetSummaryTokens := hfs.getTargetSummaryTokens()
	summaryMsgContent, keptAfterSummarization, _, err := hfs.summarizer.SummarizeMessages(
		ctx,
		currentMessages,
		targetSummaryTokens,
	)
	if err != nil {
		return currentMessages, calculateTotalTokens(currentMessages), false, fmt.Errorf(
			"failed during summarization: %w",
			err,
		)
	}
	if summaryMsgContent.Content == "" {
		return currentMessages, calculateTotalTokens(currentMessages), false, nil
	}
	return hfs.createFinalMessagesWithSummary(ctx, summaryMsgContent, keptAfterSummarization)
}

// getTargetSummaryTokens gets the target summary token count from config
func (hfs *HybridFlushingStrategy) getTargetSummaryTokens() int {
	if hfs.config != nil {
		return hfs.config.SummaryTokens
	}
	return 0
}

// createFinalMessagesWithSummary creates the final message list with summary
func (hfs *HybridFlushingStrategy) createFinalMessagesWithSummary(
	ctx context.Context,
	summaryMsgContent llm.Message,
	keptAfterSummarization []memcore.MessageWithTokens,
) ([]memcore.MessageWithTokens, int, bool, error) {
	summaryMsgWithTokens, _, err := hfs.tokenManager.CalculateMessagesWithTokens(
		ctx,
		[]llm.Message{summaryMsgContent},
	)
	if err != nil {
		summaryMsgWithTokens = []memcore.MessageWithTokens{
			{Message: summaryMsgContent, TokenCount: len(summaryMsgContent.Content) / 4},
		}
	}
	finalMessages := summaryMsgWithTokens
	finalMessages = append(finalMessages, keptAfterSummarization...)
	newTotal := calculateTotalTokens(finalMessages)
	return finalMessages, newTotal, true, nil
}

// handleSimpleFIFOFlushing handles simple FIFO flushing strategy
func (hfs *HybridFlushingStrategy) handleSimpleFIFOFlushing(
	currentMessages []memcore.MessageWithTokens,
) ([]memcore.MessageWithTokens, int, bool, error) {
	return currentMessages, calculateTotalTokens(currentMessages), false, nil
}

// handleUnsupportedFlushingType handles unsupported flushing strategy types
func (hfs *HybridFlushingStrategy) handleUnsupportedFlushingType(
	currentMessages []memcore.MessageWithTokens,
) ([]memcore.MessageWithTokens, int, bool, error) {
	return currentMessages, calculateTotalTokens(currentMessages), false, fmt.Errorf(
		"unknown or unsupported flushing strategy type: %s",
		hfs.config.Type,
	)
}

// GetType returns the strategy type
func (hfs *HybridFlushingStrategy) GetType() memcore.FlushingStrategyType {
	if hfs.config != nil {
		return hfs.config.Type
	}
	return memcore.SimpleFIFOFlushing // Default fallback
}

// PerformFlush implements the instance.FlushStrategy interface
func (hfs *HybridFlushingStrategy) PerformFlush(
	ctx context.Context,
	messages []llm.Message,
	_ *memcore.Resource,
) (*memcore.FlushMemoryActivityOutput, error) {
	// Convert llm.Message to MessageWithTokens for internal processing
	messagesWithTokens := make([]memcore.MessageWithTokens, len(messages))
	for i, msg := range messages {
		// Estimate token count if needed
		tokenCount := len(msg.Content) / 4 // Rough estimate
		if hfs.tokenManager != nil && hfs.tokenManager.tokenCounter != nil {
			if count, err := hfs.tokenManager.tokenCounter.CountTokens(ctx, msg.Content); err == nil {
				tokenCount = count
			}
		}
		messagesWithTokens[i] = memcore.MessageWithTokens{
			Message:    msg,
			TokenCount: tokenCount,
		}
	}

	// Use existing FlushMessages method
	newMessages, newTotalTokens, summaryGenerated, err := hfs.FlushMessages(ctx, messagesWithTokens)
	if err != nil {
		return &memcore.FlushMemoryActivityOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &memcore.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: summaryGenerated,
		MessageCount:     len(newMessages),
		TokenCount:       newTotalTokens,
	}, nil
}

func calculateTotalTokens(messages []memcore.MessageWithTokens) int {
	total := 0
	for _, m := range messages {
		total += m.TokenCount
	}
	return total
}
