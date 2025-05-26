package core

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
)

type ProtoObj interface {
	ToProtoBufMap() (map[string]any, error)
	ToStruct() structpb.Struct
}

func AssignProto(m map[string]any, key string, val ProtoObj) error {
	if val == nil {
		return fmt.Errorf("cannot convert nil value to protobuf map")
	}
	i, err := val.ToProtoBufMap()
	if err != nil {
		return fmt.Errorf("error converting to protobuf map: %w", err)
	}
	m[key] = i
	return nil
}

// DefaultToProtoMap converts any map with string keys to a protobuf-compatible map[string]any
func DefaultToProtoMap[V any](item map[string]V) (map[string]any, error) {
	if item == nil {
		return nil, nil
	}

	// First convert to map[string]any if V is not already any
	anyMap := make(map[string]any, len(item))
	for k, v := range item {
		anyMap[k] = v
	}

	// Then convert to protobuf-compatible values
	result := make(map[string]any)
	for k, v := range anyMap {
		val, err := structpb.NewValue(v)
		if err != nil {
			return nil, fmt.Errorf("invalid value for key %q: %w", k, err)
		}
		result[k] = val.AsInterface()
	}
	return result, nil
}
