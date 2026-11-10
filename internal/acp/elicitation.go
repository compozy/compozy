package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	// elicitationModeForm is the only elicitation mode this client renders.
	elicitationModeForm = "form"
	// elicitationOutcome* name the ACP response variants this client answers.
	elicitationOutcomeAccept  = "accept"
	elicitationOutcomeDecline = "decline"
	elicitationOutcomeCancel  = "cancel"
	// elicitationCustomSuffix marks the free-text companion opencode appends to
	// every select field ("Type your own answer"). The option list already
	// carries the answer, so the companion stays unasked.
	elicitationCustomSuffix = "_custom"
)

var (
	// ErrElicitationUnavailable reports that no operator can answer a provider
	// elicitation for this session, so the agent receives an explicit decline.
	ErrElicitationUnavailable = errors.New("acp: operator elicitation is unavailable")
	// ErrElicitationUnsupported reports a form this client cannot render.
	ErrElicitationUnsupported = errors.New("acp: elicitation form is not representable")
)

// ElicitationAsker delivers one provider question to the operator. It blocks
// until the operator answers, the context ends, or the session cannot answer.
type ElicitationAsker func(ctx context.Context, question toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error)

// wireElicitationCreateParams keeps the session id that the ACP SDK union
// drops, so a clone or foreign session cannot reach the bound operator.
type wireElicitationCreateParams struct {
	SessionID       string                           `json:"sessionId"`
	Mode            string                           `json:"mode"`
	Message         string                           `json:"message"`
	RequestedSchema acpsdk.UnstableElicitationSchema `json:"requestedSchema"`
}

// elicitationQuestion binds one rendered question to the schema keys that
// carry its answer back to the provider.
type elicitationQuestion struct {
	question  toolspkg.ClarifyQuestion
	key       string
	customKey string
	values    []string
}

// handleCreateElicitation serves the agent's inbound elicitation/create
// callback. It renders form elicitation through the operator clarification
// broker and answers decline for anything the operator cannot see.
func (p *AgentProcess) handleCreateElicitation(
	ctx context.Context,
	params json.RawMessage,
) (any, *acpsdk.RequestError) {
	var request wireElicitationCreateParams
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, acpsdk.NewInvalidParams(map[string]any{EventTypeError: err.Error()})
	}
	if !p.elicitationRequestIsBound(acpsdk.SessionId(request.SessionID)) {
		return p.declineElicitation(ctx, "unbound_session", nil), nil
	}
	if strings.TrimSpace(request.Mode) != elicitationModeForm {
		return p.declineElicitation(ctx, "unsupported_mode", nil), nil
	}
	questions, err := elicitationQuestions(request.Message, request.RequestedSchema)
	if err != nil {
		return p.declineElicitation(ctx, "unsupported_form", err), nil
	}
	answers, outcome := p.askElicitationQuestions(ctx, questions)
	if outcome != "" {
		return p.elicitationOutcome(ctx, outcome), nil
	}
	content, err := elicitationContent(questions, answers)
	if err != nil {
		return p.declineElicitation(ctx, "invalid_answer", err), nil
	}
	return acpsdk.UnstableCreateElicitationResponse{
		Accept: &acpsdk.UnstableCreateElicitationAccept{Action: elicitationOutcomeAccept, Content: content},
	}, nil
}

// askElicitationQuestions asks each rendered question in order. It returns a
// non-empty outcome when the operator could not answer the whole form.
func (p *AgentProcess) askElicitationQuestions(
	ctx context.Context,
	questions []elicitationQuestion,
) ([]toolspkg.ClarifyAnswer, string) {
	asker := p.elicitationAsker
	if asker == nil {
		p.logElicitation(ctx, slog.LevelInfo, "elicitation.declined", "unavailable")
		return nil, elicitationOutcomeDecline
	}
	askCtx := ctx
	if wait := p.permissionTimeoutOrDefault(); wait > 0 {
		var cancel context.CancelFunc
		askCtx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	answers := make([]toolspkg.ClarifyAnswer, 0, len(questions))
	for _, question := range questions {
		answer, err := asker(askCtx, question.question)
		if err == nil && answer.Fallback {
			err = context.DeadlineExceeded
		}
		if err != nil {
			outcome := p.elicitationErrorOutcome(ctx, askCtx, err)
			p.logElicitation(ctx, slog.LevelInfo, "elicitation.declined", outcome, "error", err)
			return nil, outcome
		}
		answers = append(answers, answer)
	}
	return answers, ""
}

// elicitationErrorOutcome maps an asker failure to the ACP response variant.
// A canceled turn answers cancel; every other failure answers decline so the
// provider turn keeps its named refusal instead of waiting.
func (p *AgentProcess) elicitationErrorOutcome(ctx, askCtx context.Context, err error) string {
	switch {
	case ctx.Err() != nil:
		return elicitationOutcomeCancel
	case errors.Is(err, ErrElicitationUnavailable):
		return elicitationOutcomeDecline
	case errors.Is(err, context.Canceled):
		return elicitationOutcomeCancel
	case errors.Is(err, context.DeadlineExceeded), errors.Is(askCtx.Err(), context.DeadlineExceeded):
		return elicitationOutcomeDecline
	default:
		return elicitationOutcomeDecline
	}
}

func (p *AgentProcess) declineElicitation(ctx context.Context, reason string, cause error) any {
	p.logElicitation(ctx, slog.LevelInfo, "elicitation.declined", reason, "error", cause)
	return p.elicitationOutcome(ctx, elicitationOutcomeDecline)
}

func (p *AgentProcess) elicitationOutcome(ctx context.Context, outcome string) any {
	if outcome == elicitationOutcomeCancel || ctx.Err() != nil {
		return acpsdk.UnstableCreateElicitationResponse{
			Cancel: &acpsdk.UnstableCreateElicitationCancel{Action: elicitationOutcomeCancel},
		}
	}
	return acpsdk.UnstableCreateElicitationResponse{
		Decline: &acpsdk.UnstableCreateElicitationDecline{Action: elicitationOutcomeDecline},
	}
}

func (p *AgentProcess) logElicitation(ctx context.Context, level slog.Level, message, reason string, attrs ...any) {
	logger := p.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Log(ctx, level, message, append([]any{
		"session_id", p.compozySessionID,
		"acp_session_id", string(p.boundSessionRoute()),
		"reason", reason,
	}, attrs...)...)
}

// elicitationQuestions renders a provider form as operator questions. Only
// primitive string fields are representable: a select field becomes a choice
// question and a plain string becomes a free-text question.
func elicitationQuestions(message string, schema acpsdk.UnstableElicitationSchema) ([]elicitationQuestion, error) {
	if len(schema.Properties) == 0 {
		return nil, fmt.Errorf("%w: form has no properties", ErrElicitationUnsupported)
	}
	keys := make([]string, 0, len(schema.Properties))
	for key := range schema.Properties {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	consumed := make(map[string]struct{}, len(keys))
	questions := make([]elicitationQuestion, 0, len(keys))
	for _, key := range keys {
		if _, skip := consumed[key]; skip {
			continue
		}
		property, ok := schema.Properties[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: property %q is not an object", ErrElicitationUnsupported, key)
		}
		if kind := elicitationStringField(property, "type"); kind != "string" {
			return nil, fmt.Errorf("%w: property %q has unsupported type %q",
				ErrElicitationUnsupported, key, kind)
		}
		labels, values, err := elicitationOptions(property)
		if err != nil {
			return nil, fmt.Errorf("%w: property %q: %w", ErrElicitationUnsupported, key, err)
		}
		rendered := elicitationQuestion{
			key:    key,
			values: values,
			question: toolspkg.ClarifyQuestion{
				Question: elicitationQuestionText(property, message, key),
				Choices:  labels,
			},
		}
		if len(labels) > 0 {
			custom := key + elicitationCustomSuffix
			if companion, ok := schema.Properties[custom].(map[string]any); ok &&
				elicitationStringField(companion, "type") == "string" {
				rendered.customKey = custom
				consumed[custom] = struct{}{}
			}
		}
		questions = append(questions, rendered)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("%w: form has no renderable properties", ErrElicitationUnsupported)
	}
	return questions, nil
}

// elicitationOptions reads the ACP form select shape (oneOf const/title) and
// returns parallel labels and provider values.
func elicitationOptions(property map[string]any) ([]string, []string, error) {
	raw, exists := property["oneOf"]
	if !exists {
		return nil, nil, nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil, nil, errors.New("oneOf must be an array")
	}
	if len(entries) == 0 {
		return nil, nil, nil
	}
	if len(entries) > toolspkg.MaxClarifyChoices {
		return nil, nil, fmt.Errorf("oneOf has %d options, cap is %d",
			len(entries), toolspkg.MaxClarifyChoices)
	}
	labels := make([]string, 0, len(entries))
	values := make([]string, 0, len(entries))
	for index, entry := range entries {
		option, ok := entry.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("oneOf[%d] is not an object", index)
		}
		value := elicitationScalar(option["const"])
		if value == "" {
			return nil, nil, fmt.Errorf("oneOf[%d] has no string const", index)
		}
		label := elicitationStringField(option, "title")
		if label == "" {
			label = value
		}
		labels = append(labels, label)
		values = append(values, value)
	}
	return labels, values, nil
}

// elicitationContent maps normalized answers back to the requested schema.
func elicitationContent(
	questions []elicitationQuestion,
	answers []toolspkg.ClarifyAnswer,
) (map[string]any, error) {
	if len(questions) != len(answers) {
		return nil, errors.New("answer count does not match question count")
	}
	content := make(map[string]any, len(answers))
	for index, question := range questions {
		answer := answers[index]
		if answer.Fallback {
			return nil, errors.New("fallback answer cannot be accepted")
		}
		if answer.Choice != nil {
			if len(question.values) == 0 || *answer.Choice < 0 || *answer.Choice >= len(question.values) {
				return nil, fmt.Errorf("choice %d is out of range", *answer.Choice)
			}
			content[question.key] = question.values[*answer.Choice]
			continue
		}
		text := strings.TrimSpace(answer.Text)
		if text == "" {
			return nil, fmt.Errorf("question %q has no answer", question.key)
		}
		key := question.key
		if question.customKey != "" {
			key = question.customKey
		}
		content[key] = text
	}
	return content, nil
}

func elicitationQuestionText(property map[string]any, message, key string) string {
	// description owns per-field text; message carries the whole form and is
	// the question itself for single-field forms. title is the short header.
	for _, candidate := range []string{
		elicitationStringField(property, "description"),
		strings.TrimSpace(message),
		elicitationStringField(property, "title"),
		key,
	} {
		if candidate != "" {
			return candidate
		}
	}
	return key
}

func elicitationStringField(property map[string]any, key string) string {
	value, ok := property[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func elicitationScalar(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
