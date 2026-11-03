package refs

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
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
	leftNumber, rightNumber, numeric := templateNumbers(left, right)
	if leftNumber != nil && rightNumber != nil {
		return leftNumber.Cmp(rightNumber) == 0, nil
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	switch {
	case leftValue.Kind() == reflect.Bool && rightValue.Kind() == reflect.Bool:
		return leftValue.Bool() == rightValue.Bool(), nil
	case !numeric && leftValue.Kind() == reflect.String && rightValue.Kind() == reflect.String:
		return leftValue.String() == rightValue.String(), nil
	default:
		return false, fmt.Errorf("%w: %T and %T", errIncompatibleComparison, left, right)
	}
}

func templateOrder(left any, right any) (int, error) {
	leftNumber, rightNumber, numeric := templateNumbers(left, right)
	if leftNumber != nil && rightNumber != nil {
		return leftNumber.Cmp(rightNumber), nil
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if !numeric && leftValue.Kind() == reflect.String && rightValue.Kind() == reflect.String {
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

// The flag prevents numeric operands, including invalid JSON numbers, from falling back to text.
func templateNumbers(left any, right any) (*big.Rat, *big.Rat, bool) {
	leftNumber, leftNumeric := templateNumber(left)
	rightNumber, rightNumeric := templateNumber(right)
	return leftNumber, rightNumber, leftNumeric || rightNumeric
}

func templateNumber(value any) (*big.Rat, bool) {
	number := new(big.Rat)
	if typed, ok := value.(json.Number); ok {
		return templateDecimal(typed.String()), true
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
			return nil, true
		}
		// Shortest decimal formatting can round large, exactly representable integers.
		if math.Trunc(float) == float {
			return number.SetFloat64(float), true
		}
		// Decimal formatting aligns float literals with JSON decimal values.
		return templateDecimal(strconv.FormatFloat(float, 'g', -1, reflected.Type().Bits())), true
	default:
		return nil, false
	}
}

func templateDecimal(text string) *big.Rat {
	// Bound power-of-ten allocation before the exact rational parser sees the exponent.
	const maxExponent = 4096
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		exponent, err := strconv.ParseInt(text[index+1:], 10, 32)
		if err != nil || exponent < -maxExponent || exponent > maxExponent {
			return nil
		}
	}
	if !json.Valid([]byte(text)) {
		return nil
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil
	}
	return number
}
