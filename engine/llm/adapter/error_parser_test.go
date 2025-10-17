package llmadapter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestErrorParserSuggestedRetryDelay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		provider  string
		err       error
		headers   map[string]string
		headersFn func() map[string]string
		wantSec   float64
		delta     float64
	}{
		{
			name:     "Should parse retry-after seconds from message",
			provider: "openai",
			err:      fmt.Errorf("status code: 429; retry after 2 seconds"),
			wantSec:  2,
		},
		{
			name:     "Should parse retry-after header seconds",
			provider: "openai",
			err:      fmt.Errorf("status code: 429"),
			headers:  map[string]string{"Retry-After": "3"},
			wantSec:  3,
		},
		{
			name:     "Should parse decimal seconds from message",
			provider: "groq",
			err:      fmt.Errorf("rate limit reached; please try again in 1.5s"),
			wantSec:  1.5,
			delta:    0.01,
		},
		{
			name:     "Should parse HTTP-date retry-after header",
			provider: "openai",
			err:      fmt.Errorf("status code: 429"),
			headersFn: func() map[string]string {
				return map[string]string{
					"Retry-After": time.Now().Add(3 * time.Second).In(time.FixedZone("GMT", 0)).Format(time.RFC1123),
				}
			},
			wantSec: 3,
			delta:   1.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parser := NewErrorParser(tc.provider)
			var parsed *Error
			headers := tc.headers
			if headers == nil && tc.headersFn != nil {
				headers = tc.headersFn()
			}
			if headers != nil {
				parsed = parser.ParseErrorWithHeaders(tc.err, headers)
			} else {
				parsed = parser.ParseError(tc.err)
			}
			require.NotNil(t, parsed)
			if tc.delta > 0 {
				require.InDelta(t, tc.wantSec, parsed.SuggestedRetryDelay().Seconds(), tc.delta)
			} else {
				require.Equal(t, tc.wantSec, parsed.SuggestedRetryDelay().Seconds())
			}
		})
	}
}
