# QA-NOTE-001: Repository-local QA discovery script is absent

## Status

Documented

## Severity

Low

## Discovered By

`qa-execution` repository discovery step.

## Observation

The skill recommends running `python3 scripts/discover-project-contract.py --root .`, but this repository does not currently include `scripts/discover-project-contract.py`.

## Impact

Release validation was not blocked because the same verification contract was discovered from the checked-in `Makefile`, package scripts, Playwright config, and CI-oriented test surfaces. The canonical release gate remains `make verify`.

## Evidence

- `python3 scripts/discover-project-contract.py --root .` failed with `No such file or directory`.
- `make verify` passed end-to-end after the product/test hardening fixes.
