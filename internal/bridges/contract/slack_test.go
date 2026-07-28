package contract

import (
	"reflect"
	"testing"
)

func TestSlackBotScopes(t *testing.T) {
	t.Parallel()

	t.Run("Should return the canonical sorted bot scope set", func(t *testing.T) {
		t.Parallel()

		want := []string{
			"app_mentions:read",
			"assistant:write",
			"channels:history",
			"chat:write",
			"commands",
			"groups:history",
			"im:history",
			"mpim:history",
			"reactions:read",
			"reactions:write",
		}
		if got := SlackBotScopes(); !reflect.DeepEqual(got, want) {
			t.Fatalf("SlackBotScopes() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should return a defensively owned scope slice", func(t *testing.T) {
		t.Parallel()

		first := SlackBotScopes()
		first[0] = "mutated"
		if got := SlackBotScopes()[0]; got != "app_mentions:read" {
			t.Fatalf("SlackBotScopes()[0] = %q after caller mutation, want app_mentions:read", got)
		}
	})
}
