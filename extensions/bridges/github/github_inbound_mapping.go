package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"

	"strconv"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"

	"github.com/compozy/agh/internal/subprocess"
)

func mapGitHubIssueComment(
	payload githubIssuePayload,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (githubMappedInbound, error) {
	threadType := githubThreadTypePR
	if payload.Issue.PullRequest == nil {
		threadType = githubThreadTypeIssue
	}
	threadID := encodeGitHubThreadID(githubThreadRef{
		Owner:  payload.Repository.Owner.Login,
		Repo:   payload.Repository.Name,
		Number: payload.Issue.Number,
		Type:   threadType,
	})
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		GroupID:           strings.TrimSpace(payload.Repository.FullName),
		ThreadID:          threadID,
		PlatformMessageID: strconv.FormatInt(payload.Comment.ID, 10),
		ReceivedAt:        normalizeGitHubReceivedAt(receivedAt, payload.Comment.CreatedAt),
		Sender:            githubSender(payload.Comment.User),
		Content:           bridgepkg.MessageContent{Text: payload.Comment.Body},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey:    fmt.Sprintf("github:%s:issue_comment:%d", managed.Instance.ID, payload.Comment.ID),
	}
	if metadata, err := json.Marshal(map[string]any{
		"source":                  "issue_comment",
		providerRepositoryKey:     strings.TrimSpace(payload.Repository.FullName),
		"thread_type":             threadType,
		"issue_number":            payload.Issue.Number,
		"comment_id":              payload.Comment.ID,
		providerInstallationIDKey: installationIDFromWebhook(payload.Installation),
	}); err == nil {
		envelope.ProviderMetadata = metadata
	}
	item := githubMappedInbound{
		Envelope:       envelope,
		InstallationID: installationIDFromWebhook(payload.Installation),
	}
	return item, item.Envelope.Validate()
}

func mapGitHubReviewComment(
	payload githubReviewPayload,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (githubMappedInbound, error) {
	rootCommentID := payload.Comment.ID
	if payload.Comment.InReplyToID > 0 {
		rootCommentID = payload.Comment.InReplyToID
	}
	threadID := encodeGitHubThreadID(githubThreadRef{
		Owner:           payload.Repository.Owner.Login,
		Repo:            payload.Repository.Name,
		Number:          payload.PullRequest.Number,
		Type:            githubThreadTypePR,
		ReviewCommentID: rootCommentID,
	})
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		GroupID:           strings.TrimSpace(payload.Repository.FullName),
		ThreadID:          threadID,
		PlatformMessageID: strconv.FormatInt(payload.Comment.ID, 10),
		ReceivedAt:        normalizeGitHubReceivedAt(receivedAt, payload.Comment.CreatedAt),
		Sender:            githubSender(payload.Comment.User),
		Content:           bridgepkg.MessageContent{Text: payload.Comment.Body},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey:    fmt.Sprintf("github:%s:review_comment:%d", managed.Instance.ID, payload.Comment.ID),
	}
	if metadata, err := json.Marshal(map[string]any{
		"source":                  "pull_request_review_comment",
		providerRepositoryKey:     strings.TrimSpace(payload.Repository.FullName),
		"pull_number":             payload.PullRequest.Number,
		"comment_id":              payload.Comment.ID,
		"root_review_comment_id":  rootCommentID,
		"review_comment_path":     strings.TrimSpace(payload.Comment.Path),
		providerInstallationIDKey: installationIDFromWebhook(payload.Installation),
	}); err == nil {
		envelope.ProviderMetadata = metadata
	}
	item := githubMappedInbound{
		Envelope:       envelope,
		InstallationID: installationIDFromWebhook(payload.Installation),
	}
	return item, item.Envelope.Validate()
}

func selectGitHubIssueConfig(
	candidates []resolvedInstanceConfig,
	payload githubIssuePayload,
) (resolvedInstanceConfig, bool, error) {
	return selectGitHubRoute(candidates, payload.Repository.FullName)
}

func selectGitHubReviewConfig(
	candidates []resolvedInstanceConfig,
	payload githubReviewPayload,
) (resolvedInstanceConfig, bool, error) {
	return selectGitHubRoute(candidates, payload.Repository.FullName)
}

func selectGitHubRoute(candidates []resolvedInstanceConfig, fullName string) (resolvedInstanceConfig, bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(fullName))
	if normalized == "" {
		return resolvedInstanceConfig{}, false, errors.New("github: webhook repository full_name is required")
	}
	for idx := range candidates {
		cfg := candidates[idx]
		if strings.ToLower(strings.TrimSpace(cfg.repoFullName)) == normalized {
			return cfg, true, nil
		}
	}
	return resolvedInstanceConfig{}, false, nil
}

func resolveGitHubDeliveryTarget(cfg *resolvedInstanceConfig, event bridgepkg.DeliveryEvent) (githubThreadRef, error) {
	if cfg == nil {
		return githubThreadRef{}, errors.New("github: config is required")
	}
	threadID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
	)
	if threadID == "" {
		return githubThreadRef{}, errors.New("github: delivery target requires thread_id")
	}
	target, err := decodeGitHubThreadID(threadID)
	if err != nil {
		return githubThreadRef{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.repoOwner), strings.TrimSpace(target.Owner)) ||
		!strings.EqualFold(strings.TrimSpace(cfg.repoName), strings.TrimSpace(target.Repo)) {
		return githubThreadRef{}, fmt.Errorf(
			"github: delivery target repo %q/%q does not match instance repo %q",
			target.Owner,
			target.Repo,
			cfg.repoFullName,
		)
	}
	return target, nil
}

func isGitHubSelfMessage(cfg *resolvedInstanceConfig, sender githubUser) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.botLogin) == "" {
		return false
	}
	return normalizeUsername(cfg.botLogin) == normalizeUsername(sender.Login)
}

func installationIDFromWebhook(installation *githubInstallation) int64 {
	if installation == nil {
		return 0
	}
	return installation.ID
}

func installationIDFromMetadata(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	payload := struct {
		InstallationID int64 `json:"installation_id,omitempty"`
	}{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	return payload.InstallationID
}

func githubSender(user githubUser) bridgepkg.MessageSender {
	return bridgepkg.MessageSender{
		ID:          strconv.FormatInt(user.ID, 10),
		Username:    strings.TrimSpace(user.Login),
		DisplayName: strings.TrimSpace(user.Login),
	}
}

func normalizeGitHubReceivedAt(fallback time.Time, raw string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
		return parsed.UTC()
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback.UTC()
}

func encodeGitHubThreadID(ref githubThreadRef) string {
	owner := strings.TrimSpace(ref.Owner)
	repo := strings.TrimSpace(ref.Repo)
	threadType := strings.TrimSpace(ref.Type)
	if threadType == "" {
		threadType = githubThreadTypePR
	}
	if threadType == githubThreadTypeIssue {
		return fmt.Sprintf("github:%s/%s:issue:%d", owner, repo, ref.Number)
	}
	if ref.ReviewCommentID > 0 {
		return fmt.Sprintf("github:%s/%s:%d:rc:%d", owner, repo, ref.Number, ref.ReviewCommentID)
	}
	return fmt.Sprintf("github:%s/%s:%d", owner, repo, ref.Number)
}

func decodeGitHubThreadID(value string) (githubThreadRef, error) {
	trimmed := strings.TrimSpace(value)
	if matches := githubReviewThreadPattern.FindStringSubmatch(trimmed); len(matches) == 5 {
		number, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return githubThreadRef{}, fmt.Errorf("github: parse pull number: %w", err)
		}
		reviewCommentID, err := strconv.ParseInt(matches[4], 10, 64)
		if err != nil {
			return githubThreadRef{}, fmt.Errorf("github: parse review comment id: %w", err)
		}
		return githubThreadRef{
			Owner:           matches[1],
			Repo:            matches[2],
			Number:          number,
			Type:            githubThreadTypePR,
			ReviewCommentID: reviewCommentID,
		}, nil
	}
	if matches := githubIssueThreadPattern.FindStringSubmatch(trimmed); len(matches) == 4 {
		number, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return githubThreadRef{}, fmt.Errorf("github: parse issue number: %w", err)
		}
		return githubThreadRef{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: number,
			Type:   githubThreadTypeIssue,
		}, nil
	}
	if matches := githubPRThreadPattern.FindStringSubmatch(trimmed); len(matches) == 4 {
		number, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return githubThreadRef{}, fmt.Errorf("github: parse pull number: %w", err)
		}
		return githubThreadRef{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: number,
			Type:   githubThreadTypePR,
		}, nil
	}
	return githubThreadRef{}, fmt.Errorf("github: invalid thread id %q", trimmed)
}

func encodeGitHubRemoteCommentRef(ref githubRemoteCommentRef) string {
	kind := strings.TrimSpace(ref.Kind)
	if kind == "" {
		kind = githubRemoteCommentKindIssue
	}
	return fmt.Sprintf("%s:%d", kind, ref.CommentID)
}

func parseGitHubRemoteCommentRef(value string) (githubRemoteCommentRef, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return githubRemoteCommentRef{}, errors.New("github: remote message id is required")
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return githubRemoteCommentRef{}, fmt.Errorf("github: invalid remote message id %q", trimmed)
	}
	commentID, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || commentID <= 0 {
		return githubRemoteCommentRef{}, fmt.Errorf("github: invalid remote message id %q", trimmed)
	}
	kind := strings.ToLower(strings.TrimSpace(parts[0]))
	switch kind {
	case githubRemoteCommentKindIssue, githubRemoteCommentKindReview:
		return githubRemoteCommentRef{Kind: kind, CommentID: commentID}, nil
	default:
		return githubRemoteCommentRef{}, fmt.Errorf("github: invalid remote message kind %q", kind)
	}
}

func referenceRemoteMessageID(reference *bridgepkg.DeliveryMessageReference) string {
	if reference == nil {
		return ""
	}
	return strings.TrimSpace(reference.RemoteMessageID)
}

func normalizeGitHubRepository(owner string, name string, fullName string) (string, string, string, error) {
	trimmedFull := strings.TrimSpace(fullName)
	if trimmedFull != "" {
		parts := strings.SplitN(trimmedFull, "/", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", "", "", fmt.Errorf("github: repository full_name %q must be owner/repo", trimmedFull)
		}
		return strings.TrimSpace(
				parts[0],
			), strings.TrimSpace(
				parts[1],
			), strings.TrimSpace(
				parts[0],
			) + "/" + strings.TrimSpace(
				parts[1],
			), nil
	}
	trimmedOwner := strings.TrimSpace(owner)
	trimmedName := strings.TrimSpace(name)
	if trimmedOwner == "" || trimmedName == "" {
		return "", "", "", errors.New("github: repository owner and name are required")
	}
	return trimmedOwner, trimmedName, trimmedOwner + "/" + trimmedName, nil
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func normalizeWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed != nil && parsed.Path != "" {
		trimmed = parsed.Path
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	return strings.TrimRight(parsed.String(), "/")
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func writeWebhookText(w http.ResponseWriter, body string) error {
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, body)
	return err
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func isTerminalGitHubDeliveryEvent(event bridgepkg.DeliveryEvent) bool {
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete {
		return true
	}
	switch normalizeDeliveryEventType(event.EventType) {
	case bridgepkg.DeliveryEventTypeFinal, bridgepkg.DeliveryEventTypeError, bridgepkg.DeliveryEventTypeDelete:
		return true
	default:
		return false
	}
}
