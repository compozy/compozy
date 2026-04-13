# Issue 002: GitHub install fails on unrelated archive links outside `--subdir`

## Summary

`compozy ext install --remote github --subdir ...` extracted the full GitHub tarball before narrowing to the requested extension directory. If the repository archive contained a symlink elsewhere in the repo, the install failed even though that entry was outside the requested extension subtree.

## Reproduction

```bash
HOME=/tmp/compozy-qa-remote-home \
  ./bin/compozy ext install --yes compozy/compozy \
  --remote github \
  --ref pn/ext-agents \
  --subdir extensions/cy-idea-factory
```

Observed before the fix:

- the command failed during archive extraction
- the failing tar entry was an unrelated symlink under `.compozy/tasks/_archived/...`

## Expected

When `--subdir` is provided, extraction should only materialize the requested subtree. Unrelated archive entries outside that subtree must not break the install.

## Root cause

`internal/cli/extension/install_source.go` extracted and validated every archive entry before applying the requested `--subdir`, so any unsupported entry anywhere in the repository aborted the installation.

## Fix

Apply the `--subdir` filter during tar extraction so only the requested subtree is materialized and validated.
