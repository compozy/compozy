//go:build mage

package main

import (
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
)

const perfCensusSamples = 5

type perfCensusEnvelope struct {
	Switch float64 `json:"worstSwitchFrameMs"`
	Peer   float64 `json:"worstPeerConvergenceTaskMs"`
}

// TestPerfCensus applies the unchanged 50 ms budget to the median of five
// independent E2E-023 executions. Individual samples stay in the JSON report.
func TestPerfCensus(path string) (err error) {
	defer func() {
		if err != nil {
			fmt.Printf("::error title=E2E-023 perf census::%s\n", escapeFlakeAnnotation(err.Error()))
		}
	}()
	report, err := readPlaywrightCensus(path)
	if err != nil {
		return err
	}
	samples, err := report.perfSamples()
	if err != nil {
		return fmt.Errorf("judge performance report %q: %w", path, err)
	}
	switches, peers := []float64{}, []float64{}
	for _, sample := range samples {
		switches = append(switches, sample.Switch)
		peers = append(peers, sample.Peer)
	}
	slices.Sort(switches)
	slices.Sort(peers)
	switchMedian, peerMedian := switches[perfCensusSamples/2], peers[perfCensusSamples/2]
	fmt.Printf(
		"E2E-023: samples=%d switch=%v peer=%v median_switch=%g median_peer=%g budget=50ms\n",
		len(samples),
		switches,
		peers,
		switchMedian,
		peerMedian,
	)
	if switchMedian > 50 || peerMedian > 50 {
		return fmt.Errorf(
			"median switch=%gms peer=%gms exceeds 50ms (five samples, no retries)",
			switchMedian,
			peerMedian,
		)
	}
	return nil
}

func (report playwrightCensus) perfSamples() ([]perfCensusEnvelope, error) {
	if len(report.Errors) != 0 {
		return nil, fmt.Errorf("playwright reported %d global errors", len(report.Errors))
	}
	samples := []perfCensusEnvelope{}
	for _, spec := range report.specs() {
		if !strings.Contains(spec.Title, "E2E-023: the 11-tab envelope") {
			continue
		}
		for index, test := range spec.Tests {
			sample, err := test.perfSample()
			if err != nil {
				return nil, fmt.Errorf("performance sample %d in %q (%s): %w", index+1, spec.Title, spec.File, err)
			}
			samples = append(samples, sample)
		}
	}
	if len(samples) != perfCensusSamples {
		return nil, fmt.Errorf("got %d performance samples, require %d", len(samples), perfCensusSamples)
	}
	return samples, nil
}

func (test playwrightCensusTest) perfSample() (perfCensusEnvelope, error) {
	if test.ExpectedStatus != "passed" || len(test.Results) != 1 || test.Results[0].Status != "passed" ||
		test.Results[0].Retry != 0 {
		return perfCensusEnvelope{}, fmt.Errorf(
			"every performance sample must pass its functional assertions without retries",
		)
	}
	var found *playwrightCensusAttachment
	for index, attachment := range test.Results[0].Attachments {
		if attachment.Name != "perf-envelope" {
			continue
		}
		if found != nil {
			return perfCensusEnvelope{}, fmt.Errorf("duplicate performance envelope")
		}
		found = &test.Results[0].Attachments[index]
	}
	if found == nil {
		return perfCensusEnvelope{}, fmt.Errorf("missing performance envelope")
	}
	return found.perfEnvelope()
}

func (attachment playwrightCensusAttachment) perfEnvelope() (perfCensusEnvelope, error) {
	var data []byte
	var err error
	if attachment.Path != "" {
		data, err = os.ReadFile(attachment.Path)
		if err != nil {
			return perfCensusEnvelope{}, fmt.Errorf("read performance attachment %q: %w", attachment.Path, err)
		}
	} else {
		data, err = base64.StdEncoding.DecodeString(attachment.Body)
		if err != nil {
			return perfCensusEnvelope{}, fmt.Errorf("decode base64 attachment %q: %w", attachment.Name, err)
		}
	}
	// Pointers distinguish missing measurements from a measured zero (no long tasks).
	var envelope struct {
		Switch *float64 `json:"worstSwitchFrameMs"`
		Peer   *float64 `json:"worstPeerConvergenceTaskMs"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return perfCensusEnvelope{}, fmt.Errorf(
			"decode performance attachment %q (%s): %w",
			attachment.Name,
			attachment.Path,
			err,
		)
	}
	if envelope.Switch == nil || envelope.Peer == nil {
		return perfCensusEnvelope{}, fmt.Errorf("missing performance measurements")
	}
	if *envelope.Switch < 0 || *envelope.Peer < 0 || math.IsInf(*envelope.Switch, 0) || math.IsInf(*envelope.Peer, 0) {
		return perfCensusEnvelope{}, fmt.Errorf("invalid performance measurements")
	}
	return perfCensusEnvelope{Switch: *envelope.Switch, Peer: *envelope.Peer}, nil
}
