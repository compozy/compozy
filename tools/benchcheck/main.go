package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultRegressionThreshold = 0.20

type benchmarkMetrics struct {
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  float64 `json:"bytes_per_op"`
	AllocsPerOp float64 `json:"allocs_per_op"`
}

type baselineFile struct {
	Benchmarks map[string]benchmarkMetrics `json:"benchmarks"`
}

func main() {
	baselinePath := flag.String("baseline", "", "path to baseline metrics json")
	resultsPath := flag.String("results", "", "path to benchmark output file")
	threshold := flag.Float64("threshold", defaultRegressionThreshold, "allowed relative regression before failing")
	flag.Parse()

	if err := run(*baselinePath, *resultsPath, *threshold); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(baselinePath, resultsPath string, threshold float64) error {
	if baselinePath == "" {
		return errors.New("baseline path is required")
	}
	if resultsPath == "" {
		return errors.New("results path is required")
	}
	baseline, err := loadBaseline(baselinePath)
	if err != nil {
		return err
	}
	current, err := parseBenchmarks(resultsPath)
	if err != nil {
		return err
	}
	violations := compareBenchmarks(baseline.Benchmarks, current, threshold)
	if len(violations) > 0 {
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, violation)
		}
		return errors.New("benchmark regressions detected")
	}
	fmt.Println("benchmarks within threshold")
	return nil
}

func loadBaseline(path string) (*baselineFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline %s: %w", path, err)
	}
	var file baselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse baseline %s: %w", path, err)
	}
	if len(file.Benchmarks) == 0 {
		return nil, fmt.Errorf("baseline %s contains no benchmarks", path)
	}
	return &file, nil
}

func parseBenchmarks(path string) (map[string]benchmarkMetrics, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open benchmark results %s: %w", path, err)
	}
	defer file.Close()
	metrics := make(map[string]benchmarkMetrics)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, values, ok := parseBenchmarkLine(scanner.Text())
		if !ok {
			continue
		}
		metrics[name] = values
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read benchmark results %s: %w", path, err)
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no benchmark metrics parsed from %s", path)
	}
	return metrics, nil
}

func compareBenchmarks(
	baseline map[string]benchmarkMetrics,
	current map[string]benchmarkMetrics,
	threshold float64,
) []string {
	violations := make([]string, 0)
	for name, base := range baseline {
		curr, ok := current[name]
		if !ok {
			violations = append(violations, fmt.Sprintf("benchmark %s missing from current results", name))
			continue
		}
		if exceeds(curr.NsPerOp, base.NsPerOp, threshold) {
			violations = append(
				violations,
				fmt.Sprintf("%s regressed: %.2f ns/op -> %.2f ns/op", name, base.NsPerOp, curr.NsPerOp),
			)
		}
		if exceeds(curr.BytesPerOp, base.BytesPerOp, threshold) {
			violations = append(
				violations,
				fmt.Sprintf(
					"%s allocations in bytes regressed: %.2f B/op -> %.2f B/op",
					name,
					base.BytesPerOp,
					curr.BytesPerOp,
				),
			)
		}
		if exceeds(curr.AllocsPerOp, base.AllocsPerOp, threshold) {
			violations = append(
				violations,
				fmt.Sprintf(
					"%s allocations regressed: %.2f allocs/op -> %.2f allocs/op",
					name,
					base.AllocsPerOp,
					curr.AllocsPerOp,
				),
			)
		}
	}
	for name := range current {
		if _, ok := baseline[name]; !ok {
			violations = append(violations, fmt.Sprintf("benchmark %s missing from baseline", name))
		}
	}
	return violations
}

func exceeds(value, base, threshold float64) bool {
	if base <= 0 {
		return false
	}
	return value > base*(1+threshold)
}

func parseBenchmarkLine(line string) (string, benchmarkMetrics, bool) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return "", benchmarkMetrics{}, false
	}
	nameWithSuffix := fields[0]
	name := trimBenchmarkName(nameWithSuffix)
	ns, nsErr := parseDuration(fields[2], fields[3])
	bytes, bytesErr := parseBytes(fields[4], fields[5])
	allocs, allocsErr := strconv.ParseFloat(fields[6], 64)
	if nsErr != nil || bytesErr != nil || allocsErr != nil {
		return "", benchmarkMetrics{}, false
	}
	return name, benchmarkMetrics{NsPerOp: ns, BytesPerOp: bytes, AllocsPerOp: allocs}, true
}

func trimBenchmarkName(name string) string {
	if idx := strings.LastIndex(name, "-"); idx > 0 {
		return name[:idx]
	}
	return name
}

func parseDuration(valueStr, unit string) (float64, error) {
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}
	switch unit {
	case "ns/op":
		return value, nil
	case "µs/op":
		return value * 1e3, nil
	case "ms/op":
		return value * 1e6, nil
	case "s/op":
		return value * 1e9, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit %s", unit)
	}
}

func parseBytes(valueStr, unit string) (float64, error) {
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}
	switch unit {
	case "B/op":
		return value, nil
	case "kB/op":
		return value * 1024, nil
	case "MB/op":
		return value * 1024 * 1024, nil
	case "GB/op":
		return value * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unsupported bytes unit %s", unit)
	}
}
