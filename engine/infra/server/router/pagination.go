package router

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

var cursorCodec = base64.URLEncoding.WithPadding(base64.NoPadding)

const (
	cursorPrefixV2 = "v2:"
	cursorAfter    = "after"
	cursorBefore   = "before"
)

type Cursor struct {
	Direction string
	Value     string
}

func DecodeCursor(raw string) (Cursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Cursor{}, nil
	}
	data, err := cursorCodec.DecodeString(trimmed)
	if err != nil {
		return Cursor{}, errors.New("invalid cursor")
	}
	payload := string(data)
	if !strings.HasPrefix(payload, cursorPrefixV2) {
		return Cursor{}, errors.New("invalid cursor")
	}
	parts := strings.Split(payload[len(cursorPrefixV2):], ":")
	if len(parts) != 2 {
		return Cursor{}, errors.New("invalid cursor")
	}
	direction := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if direction != cursorAfter && direction != cursorBefore {
		return Cursor{}, errors.New("invalid cursor direction")
	}
	if value == "" {
		return Cursor{}, errors.New("invalid cursor value")
	}
	return Cursor{Direction: direction, Value: value}, nil
}

func EncodeCursor(direction, value string) string {
	if direction == "" || value == "" {
		return ""
	}
	payload := fmt.Sprintf("%s%s:%s", cursorPrefixV2, direction, value)
	return cursorCodec.EncodeToString([]byte(payload))
}

// LimitOrDefault returns a sanitized page size using request-scoped configuration
// when available. It respects the provided defaults but allows operators to tune
// the default via config (CLI.PageSize) while preserving the explicit query param
// and max cap.
func LimitOrDefault(c *gin.Context, raw string, def int, maxLimit int) int {
	if c != nil {
		if cfg := config.FromContext(c.Request.Context()); cfg != nil {
			if def <= 0 && cfg.CLI.PageSize > 0 {
				def = cfg.CLI.PageSize
			}
		}
	}
	if def <= 0 {
		def = 50
	}
	if maxLimit <= 0 {
		maxLimit = 500
	}
	val, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || val <= 0 {
		return def
	}
	if val > maxLimit {
		return maxLimit
	}
	return val
}

func SetLinkHeaders(c *gin.Context, nextCursor string, prevCursor string) {
	links := make([]string, 0, 2)
	if nextCursor != "" {
		links = append(links, buildLink(c, nextCursor, "next"))
	}
	if prevCursor != "" {
		links = append(links, buildLink(c, prevCursor, "prev"))
	}
	if len(links) > 0 {
		c.Header("Link", strings.Join(links, ", "))
	}
}

func buildLink(c *gin.Context, cursor string, rel string) string {
	u, err := url.Parse(c.Request.URL.String())
	if err != nil || u == nil {
		return ""
	}
	q := u.Query()
	q.Set("cursor", cursor)
	u.RawQuery = q.Encode()
	return fmt.Sprintf("<%s>; rel=%q", sanitizedURL(u), rel)
}

func sanitizedURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	if u.Scheme == "" && u.Host == "" {
		if u.RawQuery == "" {
			return u.Path
		}
		return u.Path + "?" + u.RawQuery
	}
	return u.String()
}
