package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/CompoZy/llm-router/engine/llm"
	// Assuming logger: "github.com/CompoZy/llm-router/pkg/logger"
)

// MessageSummarizer defines an interface for summarizing a list of messages.
type MessageSummarizer interface {
	SummarizeMessages(ctx context.Context, messages []MessageWithTokens, targetSummaryTokenCount int) (llm.Message, []MessageWithTokens, error)
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
// Returns: the summary message, and the list of messages that were *not* part of the summary (i.e., those to be removed).
func (rbs *RuleBasedSummarizer) SummarizeMessages(
	ctx context.Context,
	messagesToProcess []MessageWithTokens, // These are candidates for summarization
	targetSummaryTokenCount int, // Target for the summary message itself
) (summary llm.Message,
	remainingMessages []MessageWithTokens, // Messages that are kept *after* summarization (first N, last N)
	summarizedOriginalMessages []MessageWithTokens, // Messages that were actually part of the text that got summarized
	err error) {

	if len(messagesToProcess) <= rbs.KeepFirstNMessages+rbs.KeepLastNMessages {
		// Not enough messages to apply the rule-based summarization logic,
		// or all messages are meant to be kept. No summarization needed.
		return llm.Message{}, messagesToProcess, nil, nil
	}

	var summaryContentBuilder strings.Builder
	summaryTokens := 0

	keptMessages := make([]MessageWithTokens, 0, rbs.KeepFirstNMessages+rbs.KeepLastNMessages)
	var messagesThatFormedSummaryText []MessageWithTokens


	// 1. Keep First N messages (if any)
	if rbs.KeepFirstNMessages > 0 && len(messagesToProcess) >= rbs.KeepFirstNMessages {
		keptMessages = append(keptMessages, messagesToProcess[:rbs.KeepFirstNMessages]...)
		// These are not part of the "summarized text" but are preserved alongside summary.
	}

	// 2. Identify messages for the summary text (those between first N and last N)
	// And keep Last N messages
	firstIdxForSummary := rbs.KeepFirstNMessages
	lastIdxToKeep := len(messagesToProcess) - rbs.KeepLastNMessages

	if lastIdxToKeep <= firstIdxForSummary { // Overlap or not enough messages to summarize
		return llm.Message{}, messagesToProcess, nil, nil
	}

	messagesThatFormedSummaryText = messagesToProcess[firstIdxForSummary:lastIdxToKeep]

	if rbs.KeepLastNMessages > 0 {
		keptMessages = append(keptMessages, messagesToProcess[lastIdxToKeep:]...)
	}


	// 3. Construct the summary content from messagesThatFormedSummaryText
	// For V1, the PRD states "rule-based summarization strategy ... combines the first message and the N most recent messages".
	// This implies the "summary" isn't of the middle part, but rather the overall context is maintained by keeping first+last.
	// The "hybrid flushing strategy with rule-based summarization" in Tech Spec also implies this.
	// "Summarize oldest messages when token limits are exceeded using deterministic rules"
	// "Summaries are kept in memory to maintain context"
	// "Rule-based approach combines first message and most recent messages for continuity"

	// Let's refine: The "summary message" itself might be a placeholder or a very brief note.
	// The actual "summarization" is achieved by *discarding* the middle messages.
	// The PRD for Hybrid Flushing says: "oldest messages are summarized ... and flushed. Summaries are kept."
	// Tech Spec: "Rule-based summarizer: combine first message and N most recent messages" for summary content.

	// Let's assume the "summary message" should represent the *gist* of the messages being removed.
	// For a simple rule-based summarizer, this could be:
	// "Summary: [Content of the first message that was summarized] ...and X other messages... [Content of the last message that was summarized]"

	if len(messagesThatFormedSummaryText) > 0 {
		summaryContentBuilder.WriteString(fmt.Sprintf("Summary of %d messages: ", len(messagesThatFormedSummaryText)))
		// Add content from the first summarized message
		summaryContentBuilder.WriteString(fmt.Sprintf("[%s]: %s", messagesThatFormedSummaryText[0].Role, messagesThatFormedSummaryText[0].Content))
		currentSummaryTokens, _ := rbs.tokenCounter.CountTokens(ctx, summaryContentBuilder.String())

		if len(messagesThatFormedSummaryText) > 1 {
			summaryContentBuilder.WriteString(" ... ")
			// Add content from the last summarized message if it fits
			lastSummarizedMsgContent := fmt.Sprintf("[%s]: %s", messagesThatFormedSummaryText[len(messagesThatFormedSummaryText)-1].Role, messagesThatFormedSummaryText[len(messagesThatFormedSummaryText)-1].Content)
			tokensForLast, _ := rbs.tokenCounter.CountTokens(ctx, lastSummarizedMsgContent)
			if currentSummaryTokens+tokensForLast < targetSummaryTokenCount || targetSummaryTokenCount <= 0 {
				summaryContentBuilder.WriteString(lastSummarizedMsgContent)
			}
		}
		summaryTokens, _ = rbs.tokenCounter.CountTokens(ctx, summaryContentBuilder.String())

		// Crude truncation if summary is too long (better would be to summarize the summary)
		summaryStr := summaryContentBuilder.String()
		for summaryTokens > targetSummaryTokenCount && targetSummaryTokenCount > 0 && len(summaryStr) > 0 {
			summaryStr = summaryStr[:len(summaryStr)-10] // Remove some chars
			if len(summaryStr) < 10 { summaryStr = "" } // Avoid getting stuck
			summaryTokens, _ = rbs.tokenCounter.CountTokens(ctx, summaryStr+"...")
			if summaryTokens <= targetSummaryTokenCount {
				summaryStr += "..."
			}
		}
		summaryContentBuilder.Reset()
		summaryContentBuilder.WriteString(summaryStr)
	} else {
		// No messages were actually "summarized" by removal, so no summary text.
		return llm.Message{}, messagesToProcess, nil, nil
	}

	finalSummaryMessage := llm.Message{
		Role:    "system", // Or a special "summary" role
		Content: summaryContentBuilder.String(),
		// Metadata could include info like number of messages summarized, original token count, etc.
		Metadata: map[string]interface{}{
			"summarized_message_count": len(messagesThatFormedSummaryText),
			"summary_token_count": summaryTokens,
			"summary_strategy": "rule_based_first_last",
		},
	}

	return finalSummaryMessage, keptMessages, messagesThatFormedSummaryText, nil
}


// HybridFlushingStrategy applies a flushing strategy, potentially involving summarization.
type HybridFlushingStrategy struct {
	config       *FlushingStrategyConfig
	summarizer   MessageSummarizer // e.g., RuleBasedSummarizer
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
	ctx context.Context,
	currentMessages []MessageWithTokens, // Current messages already processed for token counts
	currentTotalTokens int,
) bool {
	if hfs.config == nil {
		return false
	}

	// Determine effective max tokens for threshold calculation
	effectiveMaxTokens := hfs.tokenManager.config.MaxTokens
	if effectiveMaxTokens == 0 && hfs.tokenManager.config.MaxContextRatio > 0 {
		modelContextSize := 4096 // Placeholder, should be configurable or from model info
		effectiveMaxTokens = int(float64(modelContextSize) * hfs.tokenManager.config.MaxContextRatio)
	}

	if effectiveMaxTokens > 0 && hfs.config.SummarizeThreshold > 0 {
		thresholdTokens := int(float64(effectiveMaxTokens) * hfs.config.SummarizeThreshold)
		if currentTotalTokens >= thresholdTokens {
			// logger.Debugf(ctx, "ShouldFlush: YES. Total tokens %d >= threshold %d (%.2f of %d)",
			//	currentTotalTokens, thresholdTokens, hfs.config.SummarizeThreshold, effectiveMaxTokens)
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

		summaryMsgContent, keptAfterSummarization, _, err := hfs.summarizer.SummarizeMessages(ctx, currentMessages, targetSummaryTokens)
		if err != nil {
			return currentMessages, calculateTotalTokens(currentMessages), false, fmt.Errorf("failed during summarization: %w", err)
		}

		if summaryMsgContent.Content != "" { // A summary was actually generated
			summaryMsgWithTokens, _, _ := hfs.tokenManager.CalculateMessagesWithTokens(ctx, []llm.Message{summaryMsgContent})

			// The new message list is: summary message + messages kept by the summarizer (first N, last N)
			// The order matters: summary usually replaces the summarized part.
			// If summarizer keeps first N and last N, summary should go between them or at the start of the "older" section.
			// For simplicity, let's prepend the summary if one was made.
			finalMessages := summaryMsgWithTokens
			finalMessages = append(finalMessages, keptAfterSummarization...)

			newTotal := calculateTotalTokens(finalMessages)
			// logger.Debugf(ctx, "Flushed messages with summary. Original: %d msgs. New: %d msgs, %d tokens.", len(currentMessages), len(finalMessages), newTotal)
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

	return currentMessages, calculateTotalTokens(currentMessages), false, fmt.Errorf("unknown or unsupported flushing strategy type: %s", hfs.config.Type)
}

func calculateTotalTokens(messages []MessageWithTokens) int {
	total := 0
	for _, m := range messages {
		total += m.TokenCount
	}
	return total
}
