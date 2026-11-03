package refs

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
)

// Loop data carries numbers as json.Number, float64, or Go integers depending
// on where the value was decoded. The text/template comparison builtins compare
// json.Number as a string and reject mixed numeric kinds, so `lt .count 3`
// errors and two json.Number values order lexically ("10" < "9"). These
// curated replacements compare every numeric operand by value and keep the
// builtin semantics for strings and booleans.

var errIncompatibleComparison = errors.New("incompatible types for comparison")

func comparisonFuncs() map[string]any {
	return map[string]any{
		"eq": templateEq,
		"ne": templateNe,
		"lt": templateLt,
		"le": templateLe,
		"gt": templateGt,
		"ge": templateGe,
	}
}

// templateEq reports whether arg equals any of others, like the builtin eq.
func templateEq(arg any, others ...any) (bool, error) {
	if len(others) == 0 {
		return false, errors.New("missing argument for comparison")
	}
	for _, other := range others {
		equal, err := templateValuesEqual(arg, other)
		if err != nil {
			return false, err
		}
		if equal {
			return true, nil
		}
	}
	return false, nil
}

func templateNe(left any, right any) (bool, error) {
	equal, err := templateValuesEqual(left, right)
	return !equal, err
}

func templateLt(left any, right any) (bool, error) {
	order, err := templateOrder(left, right)
	return order < 0, err
}

func templateLe(left any, right any) (bool, error) {
	order, err := templateOrder(left, right)
	return order <= 0, err
}

func templateGt(left any, right any) (bool, error) {
	order, err := templateOrder(left, right)
	return order > 0, err
}

func templateGe(left any, right any) (bool, error) {
	order, err := templateOrder(left, right)
	return order >= 0, err
}

func templateValuesEqual(left any, right any) (bool, error) {
	if left == nil || right == nil {
		return left == nil && right == nil, nil
	}
	if leftNumber, rightNumber, ok := templateNumbers(left, right); ok {
		return leftNumber.Cmp(rightNumber) == 0, nil
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	switch {
	case leftValue.Kind() == reflect.Bool && rightValue.Kind() == reflect.Bool:
		return leftValue.Bool() == rightValue.Bool(), nil
	case leftValue.Kind() == reflect.String && rightValue.Kind() == reflect.String:
		return leftValue.String() == rightValue.String(), nil
	default:
		return false, fmt.Errorf("%w: %T and %T", errIncompatibleComparison, left, right)
	}
}

func templateOrder(left any, right any) (int, error) {
	if leftNumber, rightNumber, ok := templateNumbers(left, right); ok {
		return leftNumber.Cmp(rightNumber), nil
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if leftValue.Kind() == reflect.String && rightValue.Kind() == reflect.String {
		leftText, rightText := leftValue.String(), rightValue.String()
		switch {
		case leftText < rightText:
			return -1, nil
		case leftText > rightText:
			return 1, nil
		default:
			return 0, nil
		}
	}
	return 0, fmt.Errorf("%w: %T and %T", errIncompatibleComparison, left, right)
}

// templateNumbers compares numeric operands while preserving string comparisons for mixed operands.
func templateNumbers(left any, right any) (*big.Float, *big.Float, bool) {
	leftNumber, leftOK := templateNumber(left)
	if !leftOK {
		return nil, nil, false
	}
	rightNumber, rightOK := templateNumber(right)
	if !rightOK {
		return nil, nil, false
	}
	return leftNumber, rightNumber, true
}

func templateNumber(value any) (*big.Float, bool) {
	number := new(big.Float).SetPrec(256)
	if typed, ok := value.(json.Number); ok {
		if _, ok := number.SetString(typed.String()); !ok {
			return nil, false
		}
		return number, true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return number.SetInt64(reflected.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return number.SetUint64(reflected.Uint()), true
	case reflect.Float32, reflect.Float64:
		float := reflected.Float()
		if math.IsNaN(float) || math.IsInf(float, 0) {
			return nil, false
		}
		// Decimal formatting aligns float literals with JSON decimal values.
		return number.SetString(strconv.FormatFloat(float, 'g', -1, reflected.Type().Bits()))
	default:
		return nil, false
	}
}
