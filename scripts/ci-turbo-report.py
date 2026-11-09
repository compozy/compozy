#!/usr/bin/env python3
"""Report Turbo receipts and audit one deterministic cached test task cold."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

files = sorted(Path('.turbo/runs').glob('*.json'))
if not files:
    raise RuntimeError('No Turbo receipts recorded for this frontend lane')
tasks = []
for file in files:
    tasks.extend(json.loads(file.read_text()).get('tasks', []))
hits = [task for task in tasks if task.get('cache', {}).get('status') == 'HIT']
lines = [f'Turbo: {len(hits)} cache hits across {len(tasks)} task receipts.']
for task in tasks:
    execution = task.get('execution', {})
    start, end = execution.get('startTime'), execution.get('endTime')
    duration = f'{(end - start) / 1000:.2f}s' if isinstance(start, (int, float)) and isinstance(end, (int, float)) else 'unavailable'
    lines.append(f"- {task['taskId']}: {task.get('cache', {}).get('status', 'unknown')}; observed execution {duration}")
lines.append('Avoided runtime is not estimated without recorded cold durations.')
print('\n'.join(lines))
if os.environ.get('GITHUB_STEP_SUMMARY'):
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write('\n'.join(lines) + '\n')
candidates = {task['package'] for task in hits if task.get('task') == 'test'}
if candidates:
    seed = os.environ.get('GITHUB_SHA', '')
    package = min(candidates, key=lambda name: hashlib.sha256((seed + name).encode()).hexdigest())
    print(f'Cold cache audit: {package}#test', flush=True)
    subprocess.run(['bunx', '--no-install', 'turbo', 'run', 'test', '--filter=' + package,
                    '--cache=', '--concurrency=2'], check=True)
else:
    print('Cold cache audit: no cached test task in this lane.')
