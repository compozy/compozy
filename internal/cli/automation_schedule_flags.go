package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/spf13/cobra"
)

const (
	automationCatchUpPolicyKey       = "catch-up-policy"
	automationMisfireGraceSecondsKey = "misfire-grace-seconds"
	automationCatchUpDefaultValue    = configDefaultKey
)

type automationScheduleReliabilityInput struct {
	CatchUpPolicyRaw    string
	MisfireGraceSeconds int
}

func bindAutomationScheduleReliabilityFlags(
	cmd *cobra.Command,
	catchUpPolicyRaw *string,
	misfireGraceSeconds *int,
) {
	cmd.Flags().StringVar(
		catchUpPolicyRaw,
		automationCatchUpPolicyKey,
		"",
		"Catch-up policy for recurring schedules: default, skip_missed, coalesce, replay, or run_once_on_catchup",
	)
	cmd.Flags().IntVar(
		misfireGraceSeconds,
		automationMisfireGraceSecondsKey,
		0,
		"Missed-fire grace in seconds for recurring schedules; 0 uses the daemon jitter default",
	)
}

func automationScheduleReliabilityChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(automationCatchUpPolicyKey) ||
		cmd.Flags().Changed(automationMisfireGraceSecondsKey)
}

func applyAutomationScheduleReliability(
	cmd *cobra.Command,
	schedule *automationpkg.ScheduleSpec,
	input automationScheduleReliabilityInput,
) error {
	if schedule == nil {
		return errors.New("cli: automation schedule is required")
	}
	if cmd.Flags().Changed(automationCatchUpPolicyKey) {
		policy, err := parseAutomationCatchUpPolicy(input.CatchUpPolicyRaw)
		if err != nil {
			return err
		}
		schedule.CatchUpPolicy = policy
	}
	if cmd.Flags().Changed(automationMisfireGraceSecondsKey) {
		schedule.MisfireGraceSeconds = input.MisfireGraceSeconds
	}
	if err := schedule.Validate(automationScheduleKey); err != nil {
		return fmt.Errorf("cli: %w", err)
	}
	return nil
}

func parseAutomationCatchUpPolicy(raw string) (automationpkg.SchedulerCatchUpPolicy, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == automationCatchUpDefaultValue {
		return "", nil
	}
	policy := automationpkg.SchedulerCatchUpPolicy(value)
	if err := policy.Validate("catch_up_policy"); err != nil {
		return "", fmt.Errorf("cli: --%s: %w", automationCatchUpPolicyKey, err)
	}
	return policy, nil
}

func parseAutomationScheduleFlag(raw string) (automationpkg.ScheduleSpec, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return automationpkg.ScheduleSpec{}, errors.New("cli: --schedule is required")
	}

	lower := strings.ToLower(value)
	spec := automationpkg.ScheduleSpec{}
	switch {
	case strings.HasPrefix(lower, "every:"):
		spec.Mode = automationpkg.ScheduleModeEvery
		spec.Interval = strings.TrimSpace(value[len("every:"):])
	case strings.HasPrefix(lower, "at:"):
		timestamp, err := normalizeAutomationAtTime(strings.TrimSpace(value[len("at:"):]))
		if err != nil {
			return automationpkg.ScheduleSpec{}, err
		}
		spec.Mode = automationpkg.ScheduleModeAt
		spec.Time = timestamp
	default:
		spec.Mode = automationpkg.ScheduleModeCron
		spec.Expr = value
	}

	if err := spec.Validate(automationScheduleKey); err != nil {
		return automationpkg.ScheduleSpec{}, fmt.Errorf("cli: %w", err)
	}
	return spec, nil
}

func normalizeAutomationAtTime(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("cli: at-schedule timestamp is required")
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}

	return "", fmt.Errorf(
		"cli: invalid at-schedule timestamp %q: use RFC3339 or YYYY-MM-DDTHH:MM",
		value,
	)
}
