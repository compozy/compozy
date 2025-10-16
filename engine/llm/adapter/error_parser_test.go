package llmadapter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestErrorParserRetryAfterFromMessage(t *testing.T) {
	t.Parallel()

	parser := NewErrorParser("openai")
	err := fmt.Errorf("status code: 429; retry after 2 seconds")
	parsed := parser.ParseError(err)
	require.NotNil(t, parsed)
	require.Equal(t, time.Second*2, parsed.SuggestedRetryDelay())
}

func TestErrorParserRetryAfterFromHeaders(t *testing.T) {
	t.Parallel()

	parser := NewErrorParser("openai")
	err := fmt.Errorf("status code: 429")
	headers := map[string]string{"Retry-After": "3"}
	parsed := parser.ParseErrorWithHeaders(err, headers)
	require.NotNil(t, parsed)
	require.Equal(t, 3*time.Second, parsed.SuggestedRetryDelay())
}

func TestErrorParserRetryAfterFromMessageWithDecimal(t *testing.T) {
	t.Parallel()

	parser := NewErrorParser("groq")
	err := fmt.Errorf("rate limit reached; please try again in 1.5s")
	parsed := parser.ParseError(err)
	require.NotNil(t, parsed)
	require.InDelta(t, 1.5, parsed.SuggestedRetryDelay().Seconds(), 0.01)
}
