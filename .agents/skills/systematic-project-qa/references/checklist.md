# Systematic Project QA Checklist

Mark every item as complete before claiming the QA pass is done.

## Contract Discovery

- [ ] Root instructions and repository docs were read
- [ ] The canonical verify gate was identified or an explicit fallback was chosen
- [ ] The changed surface and regression-critical surface were identified

## Baseline

- [ ] Dependencies were installed with the repository-preferred command
- [ ] The baseline verification gate was run before scenario testing
- [ ] Any pre-existing failures were isolated with evidence

## User-Like Validation

- [ ] Changed workflows were exercised through public interfaces
- [ ] At least one unchanged regression-critical workflow was exercised
- [ ] Runtime readiness was confirmed with observable signals
- [ ] Fixtures or fake projects were realistic and minimal

## Regression Handling

- [ ] Every failure was reproduced before fixing
- [ ] Root cause was identified before implementation
- [ ] Regression coverage was added or updated when the repository supported it
- [ ] The narrow repro and impacted flows were rerun after each fix

## Final Verification

- [ ] The full verification gate was rerun after the last code change
- [ ] The most important user-like flows were rerun after the final gate
- [ ] A verification report was produced from fresh evidence
- [ ] Blocked scenarios or missing prerequisites were disclosed explicitly
