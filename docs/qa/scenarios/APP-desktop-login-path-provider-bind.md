---
id: APP-desktop-login-path-provider-bind
area: APP
title: Start a provider installed on the operator login PATH
persona: Dora
journey: J-desktop-attach-daily
expected: A desktop-owned daemon started from a GUI environment with a minimal PATH resolves the operator login-shell PATH and binds a configured provider executable available only there; operator-managed daemons retain their launcher environment, and a failed bounded lookup preserves startup with a safe warning.
entry_points: packaged desktop launch; compozy session new; compozy session prompt
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

The walk must use a real packaged Electron launch, a provider executable absent from the GUI PATH,
and a successful first prompt recorded by that provider. It must not substitute an absolute provider
command or add the provider directory to the Electron environment.
