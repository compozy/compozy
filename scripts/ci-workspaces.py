#!/usr/bin/env python3
"""Select workspace dependents, including Turbo's explicit repository inputs.

Input is a NUL-delimited git diff --name-only --no-renames -z. Unknown paths,
manifest changes and malformed metadata conservatively select every workspace.
"""
import fnmatch
import json
import sys
import subprocess
from pathlib import Path


def select(paths, root=Path('.')):
    manifest = json.loads((root / 'package.json').read_text())
    directories = manifest['workspaces']
    packages = {}
    for directory in directories:
        package = json.loads((root / directory / 'package.json').read_text())
        packages[package['name']] = (directory, package)
    selected = set()
    tasks = json.loads((root / 'turbo.json').read_text())['tasks']
    for path in paths:
        if path.endswith('package.json'):
            return sorted(packages)
        owners = {name for name, (directory, _) in packages.items()
                  if path.startswith(directory + '/')}
        for task, config in tasks.items():
            for pattern in config.get('inputs', []):
                if pattern.startswith('$TURBO_ROOT$/') and fnmatch.fnmatchcase(path, pattern[13:]):
                    if '#' not in task or task.startswith('//'):
                        return sorted(packages)
                    owners.add(task.split('#')[0])
        if not owners:
            classification = subprocess.run(
                ['bash', 'scripts/ci-changes.sh', '--classify', path],
                check=True, capture_output=True, text=True).stdout.strip()
            if classification == 'meta':
                continue
            # Backend-only paths still pass through the explicit input matching
            # above, so a Go input to site tests selects its owning workspace.
            if path.startswith(('internal/', 'cmd/', 'magefiles/', 'docs/qa/', 'docs/research/')) or path.endswith('.go'):
                continue
            return sorted(packages)
        selected.update(owners)
    while True:
        dependents = {name for name, (_, package) in packages.items()
                      if any(dependency in selected
                             for field in ('dependencies', 'devDependencies', 'peerDependencies', 'optionalDependencies')
                             for dependency in package.get(field, {}))}
        if dependents <= selected:
            return sorted(selected)
        selected.update(dependents)


if __name__ == '__main__':
    if '--lanes' in sys.argv:
        selected = json.load(sys.stdin)
        sdk = {'@compozy/site', '@compozy/client', '@compozy/extension-sdk', '@compozy/extension-react', '@compozy/example-prompt-enhancer', '@compozy/view-program-fixture'}
        lanes = ['lint']
        if '*' in selected or set(selected) - sdk - {'@compozy/desktop'}:
            lanes.append('web')
        if '*' in selected or set(selected) & sdk:
            lanes.append('site-sdk')
        print(json.dumps(lanes))
        sys.exit(0)
    try:
        paths = sys.stdin.buffer.read().decode('utf-8').split('\0')
        print(json.dumps(select([path for path in paths if path])))
    except (ValueError, KeyError, OSError, TypeError) as error:
        # A sentinel tells consumers to run without package filters.
        print(f'Workspace selection unavailable: {error}', file=sys.stderr)
        print('["*"]')
