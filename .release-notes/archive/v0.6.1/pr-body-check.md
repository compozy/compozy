---
title: Pull requests must follow the PR template
type: feat
---

For contributors: a new **PR body** check fails a pull request when its description does not follow `.github/PULL_REQUEST_TEMPLATE.md`. The check fails when:

- the **What & why**, **How you verified it** or **Impact** section is missing or empty;
- the description contains HTML comments;
- there is no `Closes #n` or `Refs #n` link and no `No issue: <reason>` line;
- the description contains local file paths or images hosted on GitHub;
- the title uses the product name without its "OS" suffix.

The template no longer contains HTML comments, so a new pull request starts from a clean body. Edit the description and the check runs again. Release pull requests and dependency-bot pull requests are exempt.

Migration notes: no action needed for CompozyOS users. Contributors with open pull requests should update their descriptions to pass the check.
