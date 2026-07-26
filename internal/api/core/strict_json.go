package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const unknownJSONFieldPrefix = "json: unknown field "

// ErrUnknownJSONField identifies request fields that are absent from the public contract.
var ErrUnknownJSONField = errors.New("unknown_field")

func normalizeStrictJSONDecodeError(err error) error {
	if err == nil {
		return nil
	}
	rawField, found := strings.CutPrefix(err.Error(), unknownJSONFieldPrefix)
	if !found {
		return err
	}
	field, unquoteErr := strconv.Unquote(strings.TrimSpace(rawField))
	if unquoteErr != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnknownJSONField, strings.TrimSpace(rawField), unquoteErr)
	}
	return fmt.Errorf("%w: %s", ErrUnknownJSONField, field)
}
