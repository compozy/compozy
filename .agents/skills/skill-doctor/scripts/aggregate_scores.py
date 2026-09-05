#!/usr/bin/env python3
"""Aggregate raw rubric scores; skill use is descriptive, never a reward."""

import argparse
import json
import sys
from pathlib import Path
from statistics import mean


def numeric(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not 0 <= value <= 1:
        raise ValueError("scores must be numbers between 0 and 1")
    return value


def aggregate(data):
    efficiency = [numeric(value) for value in data["efficiency"]]
    quality = [numeric(value) for value in data["code_quality"] if value is not None]
    if not efficiency:
        raise ValueError("at least one efficiency score is required")
    e = 0.5 + 0.5 * mean(efficiency)
    q = 0.5 + 0.5 * mean(quality) if quality else None
    return {
        "scores": {
            "efficiency": e, "code_quality": q,
            "skill_coverage": numeric(data["skill_coverage"]),
            "overall": 0.6 * e + 0.4 * q if q is not None else e,
        },
        "scored_sessions": {"efficiency": len(efficiency), "code_quality": len(quality)},
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="JSON with efficiency/code_quality arrays and skill_coverage")
    args = parser.parse_args()
    try:
        result = aggregate(json.loads(args.input.read_text()))
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"Cannot aggregate scores: {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
