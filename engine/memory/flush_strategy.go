package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/llm"
	// Assuming logger: "github.com/compozy/compozy/pkg/logger"
)

// MessageSummarizer defines an interface for summarizing a list of messages.
type MessageSummarizer interface {
	SummarizeMessages(
		ctx context.Context,
		messages []MessageWithTokens,
		targetSummaryTokenCount int,
	) (llm.Message, []MessageWithTokens, []MessageWithTokens, error)
}

// RuleBasedSummarizer implements deterministic summarization based on rules.
// For v1, it combines the first message and N most recent messages.
type RuleBasedSummarizer struct {
	tokenCounter TokenCounter
	// Configuration for how many recent messages to keep, etc.
	// For example:
	KeepFirstNMessages int // Number of initial messages to always keep (e.g. system prompts)
	KeepLastNMessages  int // Number of recent messages to keep for continuity
}

// NewRuleBasedSummarizer creates a new summarizer.
func NewRuleBasedSummarizer(counter TokenCounter, keepFirstN, keepLastN int) *RuleBasedSummarizer {
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
	}
}

// SummarizeMessages generates a summary message and identifies messages that were summarized.
// Returns: the summary message, and the list of messages that were *not* part
// of the summary (i.e., those to be removed).
func (rbs *RuleBasedSummarizer) SummarizeMessages(
	ctx context.Context,
	messagesToProcess []MessageWithTokens,
	targetSummaryTokenCount int,
) (summary llm.Message, remainingMessages []MessageWithTokens,
	summarizedOriginalMessages []MessageWithTokens, err error) {
	if !rbs.shouldSummarize(messagesToProcess) {
		return llm.Message{}, messagesToProcess, nil, nil
	}
	return rbs.performSummarization(ctx, messagesToProcess, targetSummaryTokenCount)
}

func (rbs *RuleBasedSummarizer) performSummarization(
	ctx context.Context,
	messagesToProcess []MessageWithTokens,
	targetSummaryTokenCount int,
) (llm.Message, []MessageWithTokens, []MessageWithTokens, error) {
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

func (rbs *RuleBasedSummarizer) shouldSummarize(messagesToProcess []MessageWithTokens) bool {
	return len(messagesToProcess) > rbs.KeepFirstNMessages+rbs.KeepLastNMessages
}

func (rbs *RuleBasedSummarizer) categorizeMessages(
	messagesToProcess []MessageWithTokens,
) ([]MessageWithTokens, []MessageWithTokens) {
	keptMessages := make([]MessageWithTokens, 0, rbs.KeepFirstNMessages+rbs.KeepLastNMessages)
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
	messages []MessageWithTokens,
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

func (rbs *RuleBasedSummarizer) buildBaseSummary(messages []MessageWithTokens) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Summary of %d messages: ", len(messages)))
	builder.WriteString(fmt.Sprintf("[%s]: %s", messages[0].Role, messages[0].Content))
	return builder.String()
}

func (rbs *RuleBasedSummarizer) addLastMessageToSummary(
	ctx context.Context,
	baseSummary string,
	messages []MessageWithTokens,
	targetTokenCount int,
) (string, error) {
	var builder strings.Builder
	builder.WriteString(baseSummary)
	return rbs.appendLastMessage(ctx, &builder, messages, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) appendLastMessage(
	ctx context.Context,
	builder *strings.Builder,
	messages []MessageWithTokens,
	targetTokenCount int,
) (string, error) {
	builder.WriteString(" ... ")
	lastMessage := messages[len(messages)-1]
	lastMsgContent := fmt.Sprintf("[%s]: %s", lastMessage.Role, lastMessage.Content)
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
		return len(content) / 4 // fallback estimate
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
	return rbs.performTruncation(summaryStr, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) performTruncation(summaryStr string, targetTokenCount int) string {
	if tiktokenCounter, ok := rbs.tokenCounter.(*TiktokenCounter); ok && tiktokenCounter.tke != nil {
		return rbs.truncateUsingTokenizer(summaryStr, targetTokenCount, tiktokenCounter)
	}
	return rbs.truncateUsingCharacterEstimate(summaryStr, targetTokenCount)
}

func (rbs *RuleBasedSummarizer) truncateUsingTokenizer(
	summaryStr string,
	targetTokenCount int,
	tiktokenCounter *TiktokenCounter,
) string {
	tokens := tiktokenCounter.tke.Encode(summaryStr, nil, nil)
	if len(tokens) > targetTokenCount {
		tokens = tokens[:targetTokenCount-3]
		truncatedStr := tiktokenCounter.tke.Decode(tokens)
		return truncatedStr + "..."
	}
	return summaryStr
}

func (rbs *RuleBasedSummarizer) truncateUsingCharacterEstimate(summaryStr string, targetTokenCount int) string {
	targetChars := targetTokenCount * 4
	if len(summaryStr) > targetChars {
		return summaryStr[:targetChars-3] + "..."
	}
	return summaryStr
}

// HybridFlushingStrategy applies a flushing strategy, potentially involving summarization.
type HybridFlushingStrategy struct {
	config       *FlushingStrategyConfig
	summarizer   MessageSummarizer   // e.g., RuleBasedSummarizer
	tokenManager *TokenMemoryManager // To get token counts and apply limits post-flush
}

// NewHybridFlushingStrategy creates a new hybrid flushing strategy.
func NewHybridFlushingStrategy(
	config *FlushingStrategyConfig,
	summarizer MessageSummarizer,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	if config == nil {
		return nil, fmt.Errorf("flushing strategy config cannot be nil")
	}
	if summarizer == nil && config.Type == HybridSummaryFlushing {
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

// ShouldFlush checks if flushing is needed based on current state and config.
// Uses estimated token counts and message counts for optimized check.
func (hfs *HybridFlushingStrategy) ShouldFlush(
	_ context.Context,
	_ []MessageWithTokens, // Current messages already processed for token counts
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
	// logger.Debugf(ctx, "ShouldFlush: NO. Total tokens %d < threshold (%.2f of %d)",
	//	currentTotalTokens, hfs.config.SummarizeThreshold, effectiveMaxTokens)
	return false
}

// FlushMessages applies the flushing strategy to the given messages.
// Returns the new set of messages (potentially with a summary message replacing some old ones)
// and the new total token count.
func (hfs *HybridFlushingStrategy) FlushMessages(
	ctx context.Context,
	currentMessages []MessageWithTokens, // Messages with pre-calculated token counts
) (newMessages []MessageWithTokens, newTotalTokens int, summaryGenerated bool, err error) {
	if hfs.config.Type == HybridSummaryFlushing && hfs.summarizer != nil {
		// Identify messages to summarize (e.g., oldest X% as per config)
		// For this, we need the FlushingStrategyConfig's SummarizeOldestPercent.
		// The summarizer itself might take this as a parameter or be configured with it.

		// The RuleBasedSummarizer is configured with KeepFirstN and KeepLastN.
		// It will determine which messages are "in the middle" and form the summary from them.
		targetSummaryTokens := 0
		if hfs.config != nil {
			targetSummaryTokens = hfs.config.SummaryTokens
		}

		summaryMsgContent, keptAfterSummarization, _, err := hfs.summarizer.SummarizeMessages(
			ctx,
			currentMessages,
			targetSummaryTokens,
		)
		if err != nil {
			return currentMessages, calculateTotalTokens(
					currentMessages,
				), false, fmt.Errorf(
					"failed during summarization: %w",
					err,
				)
		}

		if summaryMsgContent.Content != "" { // A summary was actually generated
			summaryMsgWithTokens, _, err := hfs.tokenManager.CalculateMessagesWithTokens(
				ctx,
				[]llm.Message{summaryMsgContent},
			)
			if err != nil {
				// Log error but continue with summary message
				summaryMsgWithTokens = []MessageWithTokens{
					{Message: summaryMsgContent, TokenCount: len(summaryMsgContent.Content) / 4},
				}
			}

			// The new message list is: summary message + messages kept by the summarizer (first N, last N)
			// The order matters: summary usually replaces the summarized part.
			// If summarizer keeps first N and last N, summary should go between them or at the start of the "older" section.
			// For simplicity, let's prepend the summary if one was made.
			finalMessages := summaryMsgWithTokens
			finalMessages = append(finalMessages, keptAfterSummarization...)

			newTotal := calculateTotalTokens(finalMessages)
			return finalMessages, newTotal, true, nil
		}
		// No summary was generated (e.g., not enough messages)
		// logger.Debug(ctx, "Summarization attempted but no summary message was generated.")
		return currentMessages, calculateTotalTokens(currentMessages), false, nil
	} else if hfs.config.Type == SimpleFIFOFlushing {
		// Simple FIFO is handled by TokenMemoryManager.EnforceLimits directly.
		// This FlushMessages might not even be called, or it could just trigger EnforceLimits.
		// For now, assume EnforceLimits handles this.
		// logger.Debug(ctx, "Simple FIFO flushing selected; handled by EnforceLimits.")
		return currentMessages, calculateTotalTokens(currentMessages), false, nil
	}

	return currentMessages, calculateTotalTokens(
			currentMessages,
		), false, fmt.Errorf(
			"unknown or unsupported flushing strategy type: %s",
			hfs.config.Type,
		)
}

func calculateTotalTokens(messages []MessageWithTokens) int {
	total := 0
	for _, m := range messages {
		total += m.TokenCount
	}
	return total
}
