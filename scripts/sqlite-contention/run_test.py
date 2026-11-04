"""Acceptance invariants owned by the paired experiment runner."""
import unittest
from unittest.mock import patch
from pathlib import Path
import json
import subprocess
import tempfile

from run import (MAX_RETENTION_SECONDS, MIN_THROUGHPUT_RATIO,
                 acceptance_failures, load_failed, throughput_comparison, trace_diagnostic)


def trial(count=100, seconds=30):
    return {"seconds": seconds, "latency": {"read": {"count": count},
            "retention_sweep": {"count": 1, "max_ms": 20000}},
            "failures": {}, "contention_diagnostics": [],
            "retention": {"DeletedEventSummaries": 20000}}


class AcceptanceTests(unittest.TestCase):
    def test_paired_ratios_tolerate_observed_runner_noise(self):
        baseline = [trial(seconds=n) for n in (30, 60, 30, 60)]
        head = [trial(seconds=n * 1.02) for n in (30, 60, 30, 60)]
        measured = throughput_comparison(baseline, head)["read"]
        self.assertGreaterEqual(measured["median_ratio"], MIN_THROUGHPUT_RATIO)
        self.assertEqual(len(measured["paired_ratios"]), 4)

    def test_sustained_loss_and_missing_operations_fail(self):
        for count in (94, 0):
            with self.subTest(count=count):
                head = [trial(count) for _ in range(4)]
                if count == 0:
                    for run in head:
                        run["latency"] = {}
                measured = throughput_comparison([trial()] * 4, head)["read"]
                self.assertLess(measured["median_ratio"], MIN_THROUGHPUT_RATIO)

    def test_ratios_are_paired_before_aggregation(self):
        measured = throughput_comparison(
            [trial(n) for n in (100, 1000, 100, 1000)],
            [trial(n) for n in (90, 1000, 90, 1000)])["read"]
        self.assertAlmostEqual(measured["median_ratio"], 0.95)

    def test_unpaired_trials_are_rejected(self):
        for baseline, head in (([], []), ([trial()], [])):
            with self.subTest(baseline=baseline, head=head):
                with self.assertRaises(ValueError):
                    throughput_comparison(baseline, head)

    def test_correctness_failures_remain_strict(self):
        self.assertFalse(load_failed(trial(), 20000))
        for key, value in (("failures", {"read": ["timeout"]}),
                           ("contention_diagnostics", [{"msg": "long hold"}]),
                           ("retention", {"DeletedEventSummaries": 19999})):
            with self.subTest(key=key):
                run = trial()
                run[key] = value
                self.assertTrue(load_failed(run, 20000))

    def test_acceptance_rejects_slow_retention_and_real_regressions(self):
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)],
                  "traced": {"head": {"failures": {"read": ["tracer timeout"]}}}}
        self.assertEqual(acceptance_failures(trials, 20000)[1:], ({}, []))
        trials["head"][0]["latency"]["retention_sweep"]["max_ms"] = MAX_RETENTION_SECONDS * 1000 + 1
        self.assertEqual(acceptance_failures(trials, 20000)[2], [0])
        trials["head"] = [trial(90) for _ in range(4)]
        self.assertIn("read", acceptance_failures(trials, 20000)[1])

    def test_acceptance_rejects_incomplete_baseline_pairs(self):
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        trials["baseline"][0]["latency"]["read"]["count"] = 0
        comparison, regressions, _ = acceptance_failures(trials, 20000)
        self.assertEqual(comparison["read"]["pair_count"], 4)
        self.assertFalse(comparison["read"]["baseline_complete"])
        self.assertIn("read", regressions)

    def test_acceptance_rejects_missing_head_samples_even_when_median_recovers(self):
        trials = {"baseline": [trial() for _ in range(4)],
                  "head": [trial(n) for n in (0, 0, 200, 200)]}
        self.assertEqual(acceptance_failures(trials, 20000)[2], [0, 1])

    def test_acceptance_rejects_head_only_operations(self):
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        for run in trials["baseline"]:
            del run["latency"]["read"]
        self.assertIn("read", acceptance_failures(trials, 20000)[1])

    def test_trace_failures_preserve_normal_acceptance_and_diagnostics(self):
        for error in (subprocess.CalledProcessError(1, "strace"),
                      RuntimeError("incomplete write accounting"), ValueError("invalid trace result"),
                      KeyError("end")):
            with self.subTest(error=error), tempfile.TemporaryDirectory() as directory:
                output = Path(directory)
                source = output / "fixture"
                source.mkdir()
                with patch("run.run", side_effect=error):
                    result = trace_diagnostic(Path("binary"), source, output, "head")
                saved = json.loads((output / "head-writes-error.json").read_text())
                self.assertEqual(saved, result)
                self.assertEqual(result["diagnostic_error"]["type"], type(error).__name__)
                self.assertFalse((output / "head-trace-data").exists())
                trials = {"baseline": [trial() for _ in range(4)],
                          "head": [trial() for _ in range(4)], "traced": {"head": result}}
                self.assertEqual(acceptance_failures(trials, 20000)[1:], ({}, []))


if __name__ == "__main__":
    unittest.main()
