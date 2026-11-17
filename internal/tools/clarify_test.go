package tools

import (
	"errors"
	"testing"
	"time"
)

func TestClarifyAnswerRequestNormalize(t *testing.T) {
	t.Parallel()

	choiceQuestion := ClarifyQuestion{Question: "Which path?", Choices: []string{"Fast", "Safe"}}
	freeQuestion := ClarifyQuestion{Question: "Which env?"}
	index := func(value int) *int { return &value }

	t.Run("Should record a choice answer with the choice source", func(t *testing.T) {
		t.Parallel()

		answer, err := ClarifyAnswerRequest{ChoiceIndex: index(1)}.Normalize(choiceQuestion)
		if err != nil {
			t.Fatalf("Normalize(choice) error = %v", err)
		}
		if answer.Choice == nil || *answer.Choice != 1 || answer.Text != "" ||
			answer.Source != ClarifyAnswerSourceChoice {
			t.Fatalf("Normalize(choice) = %#v, want choice 1 with source choice", answer)
		}
	})

	t.Run("Should record custom text on a choice question as custom, trimmed", func(t *testing.T) {
		t.Parallel()

		answer, err := ClarifyAnswerRequest{Text: "  Hybrid path  "}.Normalize(choiceQuestion)
		if err != nil {
			t.Fatalf("Normalize(custom) error = %v", err)
		}
		if answer.Choice != nil || answer.Text != "Hybrid path" || answer.Source != ClarifyAnswerSourceCustom {
			t.Fatalf("Normalize(custom) = %#v, want trimmed text with source custom", answer)
		}
	})

	t.Run("Should record free text on a question without choices as custom", func(t *testing.T) {
		t.Parallel()

		answer, err := ClarifyAnswerRequest{Text: "staging"}.Normalize(freeQuestion)
		if err != nil {
			t.Fatalf("Normalize(free text) error = %v", err)
		}
		if answer.Text != "staging" || answer.Source != ClarifyAnswerSourceCustom {
			t.Fatalf("Normalize(free text) = %#v, want staging with source custom", answer)
		}
	})

	t.Run("Should reject blank custom text and an empty answer", func(t *testing.T) {
		t.Parallel()

		for _, request := range []ClarifyAnswerRequest{{Text: "   "}, {}} {
			if _, err := request.Normalize(choiceQuestion); !errors.Is(err, ErrClarifyInvalid) {
				t.Fatalf("Normalize(%#v) error = %v, want %v", request, err, ErrClarifyInvalid)
			}
		}
	})

	t.Run("Should reject a choice combined with custom text", func(t *testing.T) {
		t.Parallel()

		_, err := ClarifyAnswerRequest{ChoiceIndex: index(0), Text: "Other"}.Normalize(choiceQuestion)
		if !errors.Is(err, ErrClarifyInvalid) {
			t.Fatalf("Normalize(combined) error = %v, want %v", err, ErrClarifyInvalid)
		}
	})

	t.Run("Should reject custom text that repeats a choice label", func(t *testing.T) {
		t.Parallel()

		_, err := ClarifyAnswerRequest{Text: "Safe"}.Normalize(choiceQuestion)
		if !errors.Is(err, ErrClarifyInvalid) {
			t.Fatalf("Normalize(repeat) error = %v, want %v", err, ErrClarifyInvalid)
		}
	})
}

func TestClarifyEventValidateAnswerSource(t *testing.T) {
	t.Parallel()

	base := func(answer *ClarifyAnswer) ClarifyEvent {
		now := time.Now().UTC()
		return ClarifyEvent{
			Status: ClarifyStatusResolved,
			Request: ClarifyPending{
				RequestID:   "request-one",
				WorkspaceID: "workspace-one",
				SessionID:   "session-one",
				AgentName:   "coder",
				Question:    "Which path?",
				Choices:     []string{"Fast", "Safe"},
				AskedAt:     now,
			},
			Answer: answer,
			At:     now,
		}
	}

	t.Run("Should accept legacy resolved answers that predate the source field", func(t *testing.T) {
		t.Parallel()

		if err := base(&ClarifyAnswer{Text: "staging"}).Validate(); err != nil {
			t.Fatalf("Validate(legacy) error = %v, want nil", err)
		}
	})

	t.Run("Should accept a source that agrees with the answer payload", func(t *testing.T) {
		t.Parallel()

		choice := 0
		if err := base(&ClarifyAnswer{Choice: &choice, Source: ClarifyAnswerSourceChoice}).Validate(); err != nil {
			t.Fatalf("Validate(choice) error = %v, want nil", err)
		}
		if err := base(&ClarifyAnswer{Text: "Other", Source: ClarifyAnswerSourceCustom}).Validate(); err != nil {
			t.Fatalf("Validate(custom) error = %v, want nil", err)
		}
	})

	t.Run("Should reject a source that contradicts the answer payload", func(t *testing.T) {
		t.Parallel()

		choice := 0
		for name, answer := range map[string]*ClarifyAnswer{
			"custom with a choice": {Choice: &choice, Source: ClarifyAnswerSourceCustom},
			"choice with text":     {Choice: &choice, Text: "Other", Source: ClarifyAnswerSourceChoice},
			"custom without text":  {Source: ClarifyAnswerSourceCustom},
			"unknown source":       {Text: "Other", Source: ClarifyAnswerSource("other")},
		} {
			if err := base(answer).Validate(); err == nil {
				t.Fatalf("Validate(%s) error = nil, want rejection", name)
			}
		}
	})
}
