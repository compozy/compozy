"""Acceptance invariants owned by the paired experiment runner."""
import unittest
from unittest.mock import patch
from pathlib import Path
import json
import subprocess
import tempfile

from run import (MAX_RETENTION_SECONDS, MIN_THROUGHPUT_RATIO,
                 acceptance_failures, load_failed, run, throughput_comparison, trace_diagnostic)


def trial(count=100, seconds=30):
    """Build a complete small trial for acceptance decisions."""
    return {"seconds": seconds, "latency": {"read": {"count": count},
            "transcript_write": {"count": count, "max_ms": 200},
            "retention_sweep": {"count": 1, "max_ms": 20000}},
            "failures": {}, "contention_diagnostics": [],
            "retention": {"DeletedEventSummaries": 20000}}


class AcceptanceTests(unittest.TestCase):
    # Invariant: original-trial diagnostics retain the same workload and receipts.
    # Owner: experiment subprocess boundary; canonical suite: AcceptanceTests.
    def test_runtime_trace_retains_original_workload_and_receipts(self):
        for enabled in (False, True):
            with self.subTest(enabled=enabled), tempfile.TemporaryDirectory() as directory:
                output = Path(directory)
                result = {"end": "2026-11-12T05:00:30Z", "warmup_start": "2026-11-12T04:59:55Z",
                          "seconds": 30, "write_commit_hooks": {"core": 60, "telemetry": 30}}

                def execute(command, **kwargs):
                    json.dump(result, kwargs["stdout"])
                    kwargs["stderr"].write(json.dumps({
                        "time": "2026-11-12T05:00:10Z", "msg": "sqlite write transaction held for at least one second",
                        "database": "core", "operation": "update session state", "commit_ms": 1030}) + "\n")

                with patch("run.subprocess.run", side_effect=execute) as child:
                    measured = run(Path("binary"), output / "fixture", "load", output, "head-0",
                                   runtime_trace=enabled)
                expected = ["binary", "-mode", "load", "-root", str(output / "fixture"), "-duration", "30s"]
                if enabled:
                    expected += ["-runtime-trace", str(output / "head-0.trace")]
                self.assertEqual(child.call_args.args[0], expected)
                self.assertEqual(measured["seconds"], 30)
                self.assertEqual(measured["transactions_per_second"], {"core": 2, "telemetry": 1})
                self.assertEqual(measured["contention_diagnostics"][0]["commit_ms"], 1030)
                self.assertEqual(json.loads((output / "head-0.json").read_text()), measured)

    # Invariant: shared I/O tails may explain transcript COMMIT warnings only
    # within the same pair; neither the median nor unrelated baseline work may
    # hide a head-only pause. Owner: runner acceptance, canonical suite: this file.
    def test_paired_commit_tails_accept_shared_io_variance(self):
        """Accept transcript COMMIT pauses explained by every matching baseline pair."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        for b, h in zip(trials["baseline"], trials["head"]):
            b["latency"]["transcript_write"]["max_ms"] = 1500
            h["latency"]["transcript_write"]["max_ms"] = 1125
            h["contention_diagnostics"] = [self.commit_pause()]
        _, regressions, invalid, tails = acceptance_failures(trials, 20000)
        self.assertEqual((regressions, invalid), ({}, []))
        self.assertFalse(tails["regression"])
        self.assertEqual(tails["median_ratio"], 0.75)

    def test_commit_pause_cannot_hide_behind_median_or_another_pair(self):
        """Reject a head-only hold even when another pair and the median look healthy."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        trials["baseline"][1]["latency"]["transcript_write"]["max_ms"] = 1800
        trials["head"][0]["contention_diagnostics"] = [self.commit_pause()]
        trials["head"][0]["latency"]["transcript_write"]["max_ms"] = 1125
        _, _, invalid, tails = acceptance_failures(trials, 20000)
        self.assertFalse(tails["regression"])
        self.assertEqual(invalid, [0])

    def test_sustained_transcript_tail_regression_fails_without_warnings(self):
        """Reject sustained tail loss independently of emitted warning records."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        for h in trials["head"]:
            h["latency"]["transcript_write"]["max_ms"] = 1100
        self.assertTrue(acceptance_failures(trials, 20000)[3]["regression"])

    def test_missing_invalid_or_failed_baseline_tail_cannot_calibrate_head(self):
        """Reject absent, nonfinite, empty or erroneous control measurements."""
        for sample in ({}, {"count": 0, "max_ms": 1500}, {"count": 100, "max_ms": float("nan")},
                       {"count": 100, "max_ms": float("inf")}, {"count": 100, "max_ms": 0}):
            with self.subTest(sample=sample):
                trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
                trials["baseline"][0]["latency"]["transcript_write"] = sample
                trials["head"][0]["contention_diagnostics"] = [self.commit_pause()]
                _, _, invalid, tails = acceptance_failures(trials, 20000)
                self.assertTrue(tails["regression"])
                self.assertEqual(invalid, [0])
        trials["baseline"][0]["latency"]["transcript_write"] = {"count": 100, "max_ms": 1500}
        trials["baseline"][0]["failures"] = {"transcript_write": ["deadline exceeded"]}
        self.assertTrue(acceptance_failures(trials, 20000)[3]["regression"])

    def test_missing_head_tail_fails_even_without_a_warning(self):
        """Require every head trial to supply a transcript tail measurement."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        del trials["head"][0]["latency"]["transcript_write"]
        _, _, invalid, tails = acceptance_failures(trials, 20000)
        self.assertEqual(invalid, [0])
        self.assertTrue(tails["regression"])

    def test_other_diagnostics_and_head_errors_remain_strict_on_slow_runner(self):
        """Keep unknown holds, checkpoint diagnostics and operation errors blocking."""
        changes = ({"callback_ms": 1000}, {"callback_ms": -1}, {"commit_ms": 999},
                   {"elapsed_ms": 1501}, {"database": "telemetry"}, {"operation": "other"},
                   {"msg": "store: passive session wal checkpoint failed"})
        for change in changes:
            with self.subTest(change=change):
                trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
                trials["baseline"][0]["latency"]["transcript_write"]["max_ms"] = 1500
                trials["head"][0]["contention_diagnostics"] = [dict(self.commit_pause(), **change)]
                self.assertEqual(acceptance_failures(trials, 20000)[2], [0])
        trials["head"][0]["contention_diagnostics"] = [self.commit_pause()]
        trials["head"][0]["failures"] = {"transcript_write": ["deadline exceeded"]}
        self.assertEqual(acceptance_failures(trials, 20000)[2], [0])

    def test_telemetry_commit_requires_its_exact_signature_and_own_storage_pair(self):
        """Shared sync pauses can explain only the measured telemetry COMMIT signature."""
        # Invariant/owner/suite: this runner's paired storage-control decision;
        # telemetry callback holds and head-only COMMIT pauses remain failures.
        warning = dict(self.commit_pause(), database="telemetry", operation="coalesced background writes")
        for baseline_max, change, expected_invalid in (
                (1500, {}, []), (1000, {}, [0]),
                (1500, {"callback_ms": 1000}, [0]),
                (1500, {"commit_ms": 999}, [0]),
                (1500, {"operation": "sweep event summaries"}, [0]),
                (1500, {"database": "core"}, [0])):
            with self.subTest(baseline_max=baseline_max, change=change):
                trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
                trials["baseline"][0]["latency"]["transcript_write"]["max_ms"] = baseline_max
                trials["head"][0]["contention_diagnostics"] = [dict(warning, **change)]
                self.assertEqual(acceptance_failures(trials, 20000)[2], expected_invalid)

    @staticmethod
    def commit_pause():
        """Return the observed successful transcript COMMIT warning signature."""
        return {"msg": "sqlite write transaction held for at least one second", "database": "session",
                "operation": "append session events and transcript", "elapsed_ms": 1125,
                "callback_ms": 5, "commit_ms": 1120}

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
        """Keep retention duration and completed-throughput regressions blocking."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)],
                  "traced": {"head": {"failures": {"read": ["tracer timeout"]}}}}
        self.assertEqual(acceptance_failures(trials, 20000)[1:3], ({}, []))
        trials["head"][0]["latency"]["retention_sweep"]["max_ms"] = MAX_RETENTION_SECONDS * 1000 + 1
        self.assertEqual(acceptance_failures(trials, 20000)[2], [0])
        trials["head"] = [trial(90) for _ in range(4)]
        self.assertIn("read", acceptance_failures(trials, 20000)[1])

    def test_acceptance_rejects_incomplete_baseline_pairs(self):
        """Require a positive baseline sample for every throughput pair."""
        trials = {"baseline": [trial() for _ in range(4)], "head": [trial() for _ in range(4)]}
        trials["baseline"][0]["latency"]["read"]["count"] = 0
        comparison, regressions, _, _ = acceptance_failures(trials, 20000)
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
        """Preserve diagnostic failures without replacing untraced acceptance results."""
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
                self.assertEqual(acceptance_failures(trials, 20000)[1:3], ({}, []))


if __name__ == "__main__":
    unittest.main()
