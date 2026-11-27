//go:build !darwin && !linux

package procutil

import "context"

func ReapRemoteConnectors(context.Context, string, string) error { return nil }
