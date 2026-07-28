package windowmanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// CanonicalRouteIntent validates and normalizes a durable application route.
func CanonicalRouteIntent(route RouteIntent) (RouteIntent, error) {
	pathname, err := canonicalRoutePathname(route.Pathname)
	if err != nil {
		return RouteIntent{}, err
	}
	if route.Search == nil {
		return RouteIntent{}, errors.New("route search must be a JSON object")
	}
	search := make(RouteSearch, len(route.Search))
	for key, raw := range route.Search {
		canonical, canonicalErr := canonicalRouteJSON(raw)
		if canonicalErr != nil {
			return RouteIntent{}, fmt.Errorf("route search key %q: %w", key, canonicalErr)
		}
		search[key] = canonical
	}
	return RouteIntent{Pathname: pathname, Search: search}, nil
}

func canonicalRoutePathname(pathname string) (string, error) {
	if pathname == "" || strings.TrimSpace(pathname) != pathname {
		return "", errors.New("route pathname must be a non-empty absolute path")
	}
	parsed, err := url.Parse(pathname)
	if err != nil {
		return "", fmt.Errorf("parse route pathname: %w", err)
	}
	if !strings.HasPrefix(pathname, "/") || strings.HasPrefix(pathname, "//") ||
		parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" {
		return "", errors.New("route pathname must be application-internal and absolute")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" ||
		strings.ContainsAny(pathname, "?#") {
		return "", errors.New("route pathname must not contain search or fragment data")
	}
	return pathname, nil
}

func canonicalRouteJSON(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("JSON value is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode JSON value: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("decode trailing JSON value: %w", err)
		}
		return nil, errors.New("multiple JSON values are not allowed")
	}
	var builder strings.Builder
	if err := appendCanonicalRouteJSON(&builder, value); err != nil {
		return nil, err
	}
	return json.RawMessage(builder.String()), nil
}

func appendCanonicalRouteJSON(builder *strings.Builder, value any) error {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool:
		if typed {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	case string:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("encode route string: %w", err)
		}
		builder.Write(encoded)
	case json.Number:
		number, err := canonicalRouteNumber(typed)
		if err != nil {
			return err
		}
		builder.WriteString(number)
	case []any:
		builder.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				builder.WriteByte(',')
			}
			if err := appendCanonicalRouteJSON(builder, item); err != nil {
				return err
			}
		}
		builder.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				builder.WriteByte(',')
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return fmt.Errorf("encode route search key: %w", err)
			}
			builder.Write(encodedKey)
			builder.WriteByte(':')
			if err := appendCanonicalRouteJSON(builder, typed[key]); err != nil {
				return err
			}
		}
		builder.WriteByte('}')
	default:
		return fmt.Errorf("unsupported route JSON value %T", value)
	}
	return nil
}

func canonicalRouteNumber(number json.Number) (string, error) {
	parsed, err := strconv.ParseFloat(number.String(), 64)
	if err != nil {
		return "", fmt.Errorf("parse route JSON number: %w", err)
	}
	if parsed == 0 {
		return "0", nil
	}
	if !strings.ContainsAny(number.String(), ".eE") {
		return number.String(), nil
	}
	canonical := strconv.FormatFloat(parsed, 'g', -1, 64)
	return strings.ReplaceAll(canonical, "e+", "e"), nil
}

// UnmarshalJSON rejects incomplete, external, or non-object route intents.
func (route *RouteIntent) UnmarshalJSON(data []byte) error {
	type routeWire struct {
		Pathname *string     `json:"pathname"`
		Search   RouteSearch `json:"search"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire routeWire
	if err := decoder.Decode(&wire); err != nil {
		return fmt.Errorf("decode route intent: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode trailing route intent: %w", err)
		}
		return errors.New("multiple route intent values are not allowed")
	}
	if wire.Pathname == nil {
		return errors.New("route pathname is required")
	}
	canonical, err := CanonicalRouteIntent(RouteIntent{Pathname: *wire.Pathname, Search: wire.Search})
	if err != nil {
		return err
	}
	*route = canonical
	return nil
}
