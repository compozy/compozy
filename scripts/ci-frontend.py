#!/usr/bin/env python3
"""Run one independent frontend lane using the selected workspace closure."""
import json
import os
import subprocess
import sys

lane = sys.argv[1]
selected = json.loads(os.environ['CI_WORKSPACES'])
all_packages = '*' in selected
site_sdk = {'@compozy/site', '@compozy/client', '@compozy/extension-sdk', '@compozy/extension-react', '@compozy/example-prompt-enhancer', '@compozy/view-program-fixture'}
packages = json.load(open('package.json'))['workspaces']
names = [json.load(open(path + '/package.json'))['name'] for path in packages]
names = [name for name in names if name != '@compozy/desktop'
         and (all_packages or name in selected)
         and ((name in site_sdk) == (lane == 'site-sdk'))]


def run(*args):
    subprocess.run(args, check=True)


def turbo(tasks, targets):
    if targets:
        run('bunx', '--no-install', 'turbo', 'run', *tasks,
            *['--filter=' + name for name in targets],
            '--concurrency=2', '--log-order=stream', '--summarize')


if lane == 'lint':
    run('bun', 'run', 'lint')
else:
    turbo(['typecheck', 'test'], names)
    turbo(['build'], [name for name in names if name in {'compozy-web', '@compozy/site'}])
    if '@compozy/client' in names:
        turbo(['test:package'], ['@compozy/client'])
    if lane == 'site-sdk' and (all_packages or any(name.startswith('@compozy/') for name in selected)):
        for package in ['sdk/typescript', 'sdk/client']:
            run('scripts/publish-sdk-package.sh', '--dry-run', package, '0.0.0-beta.0', 'beta')
    if 'compozy-web' in names:
        run('bun', 'scripts/bench-session-retention.mjs',
            'web/src/systems/session/lib/session-thread-repository.ts', '4000', '1', '16777216')
