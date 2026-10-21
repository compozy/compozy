package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/providerusage"
)

func ProviderUsageEnvelopePayload(v providerusage.Envelope) contract.ProviderUsageEnvelope {
	return contract.ProviderUsageEnvelope{
		KeychainSupported: v.KeychainSupported,
		ProfileID:         v.ProfileID,
		Revision:          v.Revision,
		GeneratedAt:       v.GeneratedAt,
		Enabled:           v.Enabled,
		Tray:              usagePointer(v.Tray, providerUsageTraySignal),
		Targets:           usageMap(v.Targets, providerUsageSnapshot),
	}
}

func providerUsageTraySignal(v providerusage.TraySignal) contract.ProviderUsageTraySignal {
	return contract.ProviderUsageTraySignal{
		ProviderID:  v.ProviderID,
		WindowID:    v.WindowID,
		Label:       v.Label,
		UsedPercent: v.UsedPercent,
		ResetsAt:    v.ResetsAt,
		Severity:    string(v.Severity),
		Stale:       v.Stale,
	}
}

func providerUsageSnapshot(v providerusage.Snapshot) contract.ProviderUsageTarget {
	return contract.ProviderUsageTarget{
		ProviderID:    v.ProviderID,
		ProviderName:  v.ProviderName,
		Status:        string(v.Status),
		Source:        usagePointer(v.Source, providerUsageSource),
		Account:       usagePointer(v.Account, providerUsageAccount),
		Windows:       usageMap(v.Windows, providerUsageWindow),
		Balances:      usageMap(v.Balances, providerUsageBalance),
		SpendCaps:     usageMap(v.SpendCaps, providerUsageSpendCap),
		Confidence:    string(v.Confidence),
		MeasuredAt:    v.MeasuredAt,
		CheckedAt:     v.CheckedAt,
		NextRefreshAt: v.NextRefreshAt,
		Refreshing:    v.Refreshing,
		Error:         usagePointer(v.Error, providerUsageUsageError),
	}
}

func providerUsageSource(v providerusage.Source) contract.ProviderUsageSource {
	return contract.ProviderUsageSource{
		Kind:  string(v.Kind),
		Label: v.Label,
	}
}

func providerUsageAccount(v providerusage.Account) contract.ProviderUsageAccount {
	return contract.ProviderUsageAccount{
		Label: v.Label,
		Plan:  v.Plan,
	}
}

func providerUsageWindow(v providerusage.Window) contract.ProviderUsageWindow {
	return contract.ProviderUsageWindow{
		ID:              v.ID,
		Label:           v.Label,
		Kind:            string(v.Kind),
		DurationSeconds: v.DurationSeconds,
		UsedPercent:     v.UsedPercent,
		Used:            usagePointer(v.Used, providerUsageAmount),
		Limit:           usagePointer(v.Limit, providerUsageAmount),
		ResetsAt:        v.ResetsAt,
		ResetText:       v.ResetText,
	}
}

func providerUsageAmount(v providerusage.Amount) contract.ProviderUsageAmount {
	return contract.ProviderUsageAmount{
		Value: v.Value,
		Unit:  v.Unit,
	}
}

func providerUsageBalance(v providerusage.Balance) contract.ProviderUsageBalance {
	return contract.ProviderUsageBalance{
		ID:        v.ID,
		Label:     v.Label,
		Remaining: providerUsageAmount(v.Remaining),
		ExpiresAt: v.ExpiresAt,
	}
}

func providerUsageSpendCap(v providerusage.SpendCap) contract.ProviderUsageSpendCap {
	return contract.ProviderUsageSpendCap{
		ID:          v.ID,
		Label:       v.Label,
		Used:        providerUsageAmount(v.Used),
		Limit:       usagePointer(v.Limit, providerUsageAmount),
		UsedPercent: v.UsedPercent,
		ResetsAt:    v.ResetsAt,
		Enabled:     v.Enabled,
	}
}

func providerUsageUsageError(v providerusage.UsageError) contract.ProviderUsageError {
	return contract.ProviderUsageError{
		Code:       string(v.Code),
		Message:    v.Message,
		Action:     v.Action,
		RetryAfter: v.RetryAfter,
	}
}

func usageMap[A, B any](items []A, convert func(A) B) []B {
	result := make([]B, len(items))
	for i, item := range items {
		result[i] = convert(item)
	}
	return result
}
func usagePointer[A, B any](value *A, convert func(A) B) *B {
	if value == nil {
		return nil
	}
	result := convert(*value)
	return &result
}
