package mcp

import (
	"errors"
	"testing"
)

func TestRuntimeHealthRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the newest outcome and clear on success", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		key := RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-a", ServerName: "deployment-api",
		}
		oldFailure := registry.Begin(key)
		newSuccess := registry.Begin(key)
		registry.RecordSuccess(newSuccess)
		registry.RecordFailure(oldFailure, errors.New("stale connection failure"))
		if entries := registry.Entries("kit", "", "generation-a"); len(entries) != 0 {
			t.Fatalf("Entries() = %#v, want newer success to win", entries)
		}

		currentFailure := registry.Begin(key)
		registry.RecordFailure(currentFailure, errors.New("current connection failure"))
		entries := registry.Entries("kit", "", "generation-a")
		if len(entries) != 1 || entries[0].Key.ServerName != "deployment-api" ||
			entries[0].Message != "current connection failure" {
			t.Fatalf("Entries() = %#v, want current deployment-api failure", entries)
		}

		recovery := registry.Begin(key)
		registry.RecordSuccess(recovery)
		if entries := registry.Entries("kit", "", "generation-a"); len(entries) != 0 {
			t.Fatalf("Entries() after recovery = %#v, want empty", entries)
		}
	})

	t.Run("Should isolate instances generations and server siblings", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		keys := []RuntimeHealthKey{
			{InstanceName: "kit", WorkspaceID: "workspace-a", BundleGeneration: "generation-a", ServerName: "alpha"},
			{InstanceName: "kit", WorkspaceID: "workspace-a", BundleGeneration: "generation-a", ServerName: "beta"},
			{InstanceName: "kit", WorkspaceID: "workspace-b", BundleGeneration: "generation-a", ServerName: "alpha"},
			{InstanceName: "kit", WorkspaceID: "workspace-a", BundleGeneration: "generation-b", ServerName: "alpha"},
		}
		for _, key := range keys {
			observation := registry.Begin(key)
			registry.RecordFailure(observation, errors.New("unavailable"))
		}
		entries := registry.Entries("kit", "workspace-a", "generation-a")
		if len(entries) != 2 || entries[0].Key.ServerName != "alpha" || entries[1].Key.ServerName != "beta" {
			t.Fatalf("Entries(workspace-a, generation-a) = %#v, want sorted alpha and beta", entries)
		}
	})

	t.Run("Should evict an instance and fence in-flight observations", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		key := RuntimeHealthKey{
			InstanceName: "kit", WorkspaceID: "workspace-a", BundleGeneration: "generation-a", ServerName: "remote",
		}
		stale := registry.Begin(key)
		visible := registry.Begin(key)
		registry.RecordFailure(visible, errors.New("visible failure"))
		registry.EvictInstance("kit", "workspace-a")
		registry.RecordFailure(stale, errors.New("late failure"))
		if entries := registry.Entries("kit", "workspace-a", "generation-a"); len(entries) != 0 {
			t.Fatalf("Entries() after eviction = %#v, want empty", entries)
		}

		fresh := registry.Begin(key)
		registry.RecordFailure(fresh, errors.New("fresh failure"))
		if entries := registry.Entries(
			"kit",
			"workspace-a",
			"generation-a",
		); len(entries) != 1 ||
			entries[0].Message != "fresh failure" {
			t.Fatalf("Entries() after fresh observation = %#v, want fresh failure", entries)
		}
	})

	t.Run("Should start empty on a new registry", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		observation := registry.Begin(RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-a", ServerName: "remote",
		})
		registry.RecordFailure(observation, errors.New("failure before restart"))

		restarted := NewRuntimeHealthRegistry()
		if entries := restarted.Entries("kit", "", "generation-a"); len(entries) != 0 {
			t.Fatalf("Entries() after restart = %#v, want empty", entries)
		}
	})
}
