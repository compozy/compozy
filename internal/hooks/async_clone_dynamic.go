package hooks

import (
	"encoding/json"
	"reflect"
)

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		return cloneAnySlice(typed)
	case map[string]string:
		return cloneStringMap(typed)
	case []string:
		return cloneStringSlice(typed)
	case []byte:
		return cloneRawMessage(typed)
	case json.RawMessage:
		return cloneRawJSON(typed)
	default:
		if cloned, ok := cloneDynamicContainer(reflect.ValueOf(value)); ok {
			return cloned.Interface()
		}
		return value
	}
}

func cloneDynamicContainer(value reflect.Value) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		cloned, ok := cloneDynamicContainer(value.Elem())
		if !ok {
			cloned = value.Elem()
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(cloned)
		return out, true
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		cloned, ok := cloneDynamicContainer(value.Elem())
		if !ok {
			return value, false
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloned)
		return out, true
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			mapValue := iter.Value()
			clonedValue, ok := cloneDynamicContainer(mapValue)
			if !ok {
				clonedValue = mapValue
			}
			out.SetMapIndex(iter.Key(), clonedValue)
		}
		return out, true
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i)
			clonedItem, ok := cloneDynamicContainer(item)
			if !ok {
				clonedItem = item
			}
			out.Index(i).Set(clonedItem)
		}
		return out, true
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i)
			clonedItem, ok := cloneDynamicContainer(item)
			if !ok {
				clonedItem = item
			}
			out.Index(i).Set(clonedItem)
		}
		return out, true
	default:
		return value, false
	}
}
