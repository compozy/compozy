package session

import (
	"errors"
	"strings"
	"testing"
)

func TestGoalCommandParserShouldRecognizeOnlyAuthenticatedIngressGrammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		message       string
		wantMatched   bool
		wantVerb      string
		wantExpected  string
		wantObjective string
		wantReason    GoalReasonCode
	}{
		{
			name:          "Should parse a free-form objective",
			message:       "/goal ship release",
			wantMatched:   true,
			wantVerb:      "set",
			wantObjective: "ship release",
		},
		{
			name:          "Should parse an explicit set objective without preserving the verb",
			message:       "/goal set ship release",
			wantMatched:   true,
			wantVerb:      "set",
			wantObjective: "ship release",
		},
		{
			name:          "Should parse expected replacement",
			message:       "/goal replace run-1 ship safer",
			wantMatched:   true,
			wantVerb:      "replace",
			wantExpected:  "run-1",
			wantObjective: "ship safer",
		},
		{name: "Should parse status", message: "/goal status", wantMatched: true, wantVerb: "status"},
		{
			name:          "Should parse draft",
			message:       "/goal draft improve this",
			wantMatched:   true,
			wantVerb:      "draft",
			wantObjective: "improve this",
		},
		{name: "Should pass through a neighboring prefix", message: "/goalx ship", wantMatched: false},
		{name: "Should pass through a mid-message token", message: "please /goal ship", wantMatched: false},
		{
			name:          "Should treat an unknown word as objective text",
			message:       "/goal frobnicate safely",
			wantMatched:   true,
			wantVerb:      "set",
			wantObjective: "frobnicate safely",
		},
		{
			name:        "Should reject missing objective",
			message:     "/goal",
			wantMatched: true,
			wantReason:  GoalReasonObjectiveRequired,
		},
		{
			name:        "Should reject an explicit set without an objective",
			message:     "/goal set",
			wantMatched: true,
			wantReason:  GoalReasonObjectiveRequired,
		},
		{
			name:        "Should reject malformed replacement",
			message:     "/goal replace run-1",
			wantMatched: true,
			wantReason:  GoalReasonCommandInvalid,
		},
		{
			name:        "Should reject arguments on status",
			message:     "/goal status now",
			wantMatched: true,
			wantReason:  GoalReasonCommandInvalid,
		},
		{
			name:        "Should reject oversized objective by Unicode code point",
			message:     "/goal " + strings.Repeat("界", maxGoalCommandCodePoints+1),
			wantMatched: true,
			wantReason:  GoalReasonObjectiveTooLarge,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			command, matched, err := ParseGoalCommand(tc.message)
			if matched != tc.wantMatched {
				t.Fatalf("ParseGoalCommand() matched = %v, want %v", matched, tc.wantMatched)
			}
			if tc.wantReason != "" {
				var commandErr *GoalCommandError
				if !errors.As(err, &commandErr) || commandErr.Code != tc.wantReason {
					t.Fatalf("ParseGoalCommand() error = %v, want %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGoalCommand() error = %v", err)
			}
			if command.Verb != tc.wantVerb || command.ExpectedRunID != tc.wantExpected ||
				command.Objective != tc.wantObjective {
				t.Fatalf("ParseGoalCommand() = %#v", command)
			}
		})
	}
}

func TestGoalObjectiveParserShouldKeepClausesTextual(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve ordered textual verify and constraints clauses", func(t *testing.T) {
		t.Parallel()

		contract, err := ParseGoalObjective(
			"Ship safely\r\nverify: make verify\r\nconstraints: never execute this text\nverify: inspect evidence",
		)
		if err != nil {
			t.Fatalf("ParseGoalObjective() error = %v", err)
		}
		if contract.Objective != "Ship safely" ||
			strings.Join(contract.Verify, "|") != "make verify|inspect evidence" ||
			strings.Join(contract.Constraints, "|") != "never execute this text" {
			t.Fatalf("ParseGoalObjective() = %#v", contract)
		}
	})

	t.Run("Should reject an empty clause", func(t *testing.T) {
		t.Parallel()

		_, err := ParseGoalObjective("Ship safely\nverify:")
		var commandErr *GoalCommandError
		if !errors.As(err, &commandErr) || commandErr.Code != GoalReasonContractClauseEmpty {
			t.Fatalf("ParseGoalObjective() error = %v", err)
		}
	})
}
