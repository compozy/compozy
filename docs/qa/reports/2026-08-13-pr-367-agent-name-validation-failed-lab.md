---
date: 2026-08-13
status: fail
scenarios:
  - MS-web-agent-create-simple-advanced
---

# PR #367 failed QA lab

Lab: `compozy-pr-367-agent-name-validation-20260813-184620-377506`.

The desktop Create agent dialog was opened through the public Global entry point. The Global footer
read `Creates in Global — visible to every workspace.` The invalid name `audio designer` remained
visible, displayed its inline validation error, disabled Create, and produced no create request.

A 106-character canonical name enabled Create. Its public browser request was:

```json
{
  "scope": "global",
  "agent": {
    "name": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "prompt": "Review a pull request and propose focused changes."
  }
}
```

The public response was HTTP 400:

```json
{
  "error": "api: invalid create agent request\nagent runtime cannot be resolved in the target scope: agent provider is required; run `compozy install` or set agent.provider/defaults.provider"
}
```

The request omitted `workspace`, so the Global destination contract held. The failure prevents the
boundary-valid creation proof. The failed lab must not be reused.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-367-agent-name-validation-20260813-184620-377506-lab/qa-artifacts/qa/evidence/agent-invalid-whitespace-clean.png`; browser network request `68618.2060`.
