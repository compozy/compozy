package windowmanager

import "math"

func equalWeights(count int) []float64 {
	values := make([]float64, count)
	for index := range values {
		values[index] = 1 / float64(count)
	}
	return values
}

func weightsEqual(weights []float64) bool {
	if len(weights) == 0 {
		return true
	}
	expected := 1 / float64(len(weights))
	for _, weight := range weights {
		if math.Abs(weight-expected) > weightTolerance {
			return false
		}
	}
	return true
}
