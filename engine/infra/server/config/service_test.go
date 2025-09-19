package config

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/webhook"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/require"
)

func TestCompileFromStore_WithMemoryStore(t *testing.T) {
	t.Run("Should compile workflows from store", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		svc := &service{envFilePath: "", store: store}
		proj := &project.Config{Name: "demo"}
		key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceWorkflow, ID: "wf1"}
		_, err := store.Put(ctx, key, &wf.Config{ID: "wf1"})
		require.NoError(t, err)
		compiled, err := svc.compileFromStore(ctx, proj, nil)
		require.NoError(t, err)
		require.Len(t, compiled, 1)
		require.Equal(t, "wf1", compiled[0].ID)
	})
}

func TestCompileFromStore_WebhookSlugValidation(t *testing.T) {
	t.Run("Should fail on duplicate webhook slugs", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		svc := &service{envFilePath: "", store: store}
		proj := &project.Config{Name: "demo"}
		key1 := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceWorkflow, ID: "wfA"}
		key2 := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceWorkflow, ID: "wfB"}
		w1 := &wf.Config{
			ID:       "wfA",
			Triggers: []wf.Trigger{{Type: wf.TriggerTypeWebhook, Webhook: &webhook.Config{Slug: "dup"}}},
		}
		w2 := &wf.Config{
			ID:       "wfB",
			Triggers: []wf.Trigger{{Type: wf.TriggerTypeWebhook, Webhook: &webhook.Config{Slug: "dup"}}},
		}
		_, err := store.Put(ctx, key1, w1)
		require.NoError(t, err)
		_, err = store.Put(ctx, key2, w2)
		require.NoError(t, err)
		_, err = svc.compileFromStore(ctx, proj, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "webhook")
		require.ErrorContains(t, err, "invalid")
		require.ErrorContains(t, err, "duplicate")
	})
}
