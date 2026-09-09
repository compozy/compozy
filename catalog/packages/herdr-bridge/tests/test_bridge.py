"""Behavioral regressions for the catalog bridge's Python runtime.

Owning layer: extension hook consumer, row store, and event renderer.
Only socket/HTTP I/O is mocked; a process-launch wrapper reaps the real hook drainer.
"""
import contextlib
import io
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import bridge
import bridge_loops
import bridge_state
import colorize
import tail


class BridgeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        state = Path(self.temp.name)
        for name, value in {'STATE_DIR': str(state), 'MAP_PATH': str(state / 'panes.json'),
                            'LOG_PATH': str(state / 'bridge.log')}.items():
            p = patch.object(bridge_state, name, value)
            p.start()
            self.addCleanup(p.stop)
        self.spool = state / 'spool'
        self.spool.mkdir()
        p = patch.object(bridge, 'SPOOL_DIR', str(self.spool))
        p.start()
        self.addCleanup(p.stop)

    def test_outage_preserves_rows_and_does_not_create_duplicate_tabs(self):
        entry = {'pane_id': 'w1:p1', 'tab_id': 'w1:t1', 'sessions': {}}
        loop = {**entry, 'kind': 'loop', 'run_id': 'run1'}
        data = {'ws/agent': entry, 'loop/no-ws/loop': loop}
        bridge_state.save_map(data)
        for response in (None, {'error': {'code': 'unavailable'}}, {'result': {}}):
            with self.subTest(response=response), patch.object(bridge_state, 'herdr', return_value=response), \
                    patch.object(bridge, 'herdr') as rpc, patch.object(bridge_loops, 'herdr') as loop_rpc:
                with contextlib.redirect_stdout(io.StringIO()):
                    bridge.cmd_status()
                self.assertEqual(bridge_state.load_map(), data)
                self.assertIs(bridge.ensure_row(data, 'ws/agent', 'agent'), entry)
                self.assertIs(bridge_loops.ensure_loop_row(data, 'loop/no-ws/loop', 'loop', 'run1'), loop)
                self.assertFalse(any(c.args[0] == 'tab.create' for c in rpc.call_args_list + loop_rpc.call_args_list))
        with patch.object(bridge_state, 'herdr', return_value={'error': {'code': 'pane_not_found'}}):
            self.assertEqual(set(bridge_state.prune(data)), {'ws/agent', 'loop/no-ws/loop'})
            self.assertEqual(data, {})

    def test_missing_workspace_never_becomes_a_cli_workspace_argument(self):
        for workspace in (None, 'no-ws', 'ws-real'):
            args = shlex.split(bridge_loops.loop_tail_command('run1', workspace))
            self.assertEqual(args, ['compozy', 'loop', 'events', 'run1', '--follow'] +
                             (['--workspace', workspace] if workspace == 'ws-real' else []))

    def test_spool_orders_nanoseconds_before_consolidating_activity(self):
        base = {'session_type': 'user', 'agent_name': 'agent', 'session_id': 'sid', 'workspace_id': 'ws'}
        events = [('2026-09-09T12:00:00.000000002Z', 'turn.end'),
                  ('2026-09-09T12:00:00.000000001Z', 'permission.request'),
                  ('2026-09-09T09:00:00-03:00', 'turn.start')]
        for index, (stamp, event) in enumerate(events):
            (self.spool / f'{index}.json').write_text(json.dumps({**base, 'timestamp': stamp, 'event': event}))
        bridge_state.save_map({'ws/agent': {'pane_id': 'p', 'tab_id': 't', 'sessions': {}}})
        with patch.object(bridge_state, 'herdr', return_value={'result': {}}), patch.object(bridge, 'herdr') as rpc:
            self.assertEqual(bridge.drain_spool(), 3)
        self.assertEqual([c.args[1]['state'] for c in rpc.call_args_list if c.args[0] == 'pane.report_agent'],
                         ['working', 'blocked', 'idle'])
        self.assertEqual(bridge_state.load_map()['ws/agent']['sessions']['sid']['state'], 'idle')
        self.assertEqual(bridge.drain_spool(), 0)
        self.assertEqual(list(self.spool.iterdir()), [])

    def test_no_workspace_uses_authoritative_session_owner_and_retries(self):
        renderer = colorize.Renderer()
        reader = tail.SessionTail('no-ws/agent', renderer)
        data = {'no-ws/agent': {'sessions': {'sid': {}}}}
        event = {'sequence': 1, 'type': 'agent_message', 'content': {'text': 'hello world'}}
        with patch.object(tail, 'get_json', side_effect=[OSError('offline')]):
            reader.poll(data)
        self.assertIsNone(reader.cursors['sid'])
        out = io.StringIO()
        with patch.object(tail, 'get_json', side_effect=[{'workspace_id': 'ws-real', 'session_id': 'sid'},
                                                       {'events': [event]}]) as get, contextlib.redirect_stdout(out):
            reader.poll(data)
        self.assertEqual(get.call_args_list[0].args[0], '/api/sessions/sid/owner')
        self.assertIn('/api/workspaces/ws-real/sessions/sid/events?', get.call_args_list[1].args[0])
        self.assertIn('hello world', out.getvalue())
        self.assertEqual(reader.cursors['sid'], 1)
        with patch.object(tail, 'get_json', return_value={'workspace_id': 'wrong', 'session_id': 'other'}) as get:
            reader.poll(data)
        self.assertEqual(get.call_count, 1)
        self.assertEqual(reader.cursors['sid'], 1)

    def test_explicit_workspace_and_final_batch_keep_scope_and_cursor(self):
        reader = tail.SessionTail('ws/agent', colorize.Renderer())
        reader.cursors['sid'] = 1
        with patch.object(tail, 'get_json', return_value={'events': []}) as get:
            reader.poll({})
        self.assertEqual(get.call_args.args[0], '/api/workspaces/ws/sessions/sid/events?archive=all&after_sequence=1')
        self.assertEqual(reader.cursors, {})

    def test_terminal_controls_are_escaped_across_fragments_and_fallbacks(self):
        renderer = colorize.Renderer()
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            for text in ('hello \x1b', ']52;c;secret\x07\n\tworld\r\x9b2J'):
                renderer.feed({'type': 'agent_message', 'summary': text})
            renderer.feed({'type': '\x1b[2J', 'timestamp': 'x'*11+'\x07'*8, 'summary': 'body\x00'})
            renderer.close_stream()
        rendered = out.getvalue()
        for forbidden in ('\x1b]52', '\x07', '\r', '\x9b', '\x00', '\x1b[2J'):
            self.assertNotIn(forbidden, rendered)
        self.assertIn('hello \\x1b]52;c;secret\\x07\n\tworld\\x0d\\x9b2J', rendered)
        with patch.object(sys, 'stdin', io.StringIO('fallback\x1b]52;c;x\x07\n')), contextlib.redirect_stdout(out):
            colorize.main()
        self.assertIn('fallback\\x1b]52;c;x\\x07', out.getvalue())

        result = subprocess.run([sys.executable, '-B', str(Path(colorize.__file__))],
                                input='{broken\n' + json.dumps({'type': 'agent_message', 'summary': 'next event'}) + '\n',
                                capture_output=True, text=True, check=True)
        self.assertIn('cannot render event: JSONDecodeError', result.stderr)
        self.assertIn('next event', result.stdout)

    def test_hook_spool_is_private_and_xdg_drainer_consumes_it(self):
        root = Path(__file__).resolve().parents[1]
        xdg = Path(self.temp.name) / 'xdg'
        spool = xdg / 'herdr-bridge' / 'spool'
        spool.mkdir(parents=True, mode=0o755)
        spool.chmod(0o755)
        completion = Path(self.temp.name) / 'drainer-complete'
        launchers = Path(self.temp.name) / 'bin'
        launchers.mkdir()
        wrapper = launchers / 'nohup'
        # The shell waits for and reaps the real drainer before signaling completion.
        wrapper.write_text('#!/bin/sh\n"$@"\nresult=$?\nprintf "%s" "$result" > "$BRIDGE_TEST_COMPLETION"\n')
        wrapper.chmod(0o755)
        env = {**os.environ, 'XDG_STATE_HOME': str(xdg), 'HOME': self.temp.name,
               'PATH': str(launchers) + os.pathsep + os.environ['PATH'],
               'BRIDGE_TEST_COMPLETION': str(completion), 'PYTHONDONTWRITEBYTECODE': '1'}
        # Hold the production drain lock so payload permissions can be observed.
        import fcntl
        with (spool.parent / '.drain-lock').open('w') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            subprocess.run(['sh', str(root / 'hook.sh')], input='{"event":"unknown"}', text=True,
                           check=True, env=env, umask=0o022)
            files = list(spool.glob('*.json'))
            self.assertEqual(len(files), 1)
            self.assertEqual(files[0].stat().st_mode & 0o777, 0o600)
            self.assertEqual(spool.stat().st_mode & 0o777, 0o700)
        deadline = time.monotonic() + 5
        while not completion.exists() and time.monotonic() < deadline:
            time.sleep(0.05)
        self.assertTrue(completion.exists(), 'detached drainer did not finish')
        self.assertEqual(completion.read_text(), '0')
        self.assertEqual(list(spool.glob('*.json')), [])

    def test_corrupt_state_is_preserved_and_command_failure_is_logged(self):
        state = Path(self.temp.name) / 'herdr-bridge'
        state.mkdir()
        map_path = state / 'panes.json'
        map_path.write_text('{broken')
        command = Path(__file__).resolve().parents[1] / 'bridge.py'
        result = subprocess.run([sys.executable, '-B', str(command), '--status'],
                                env={**os.environ, 'XDG_STATE_HOME': self.temp.name}, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn(b"cannot recover bridge map", result.stderr)
        self.assertEqual(map_path.read_text(), '{broken')
        self.assertIn('unexpected bridge failure:', (state / 'bridge.log').read_text())

    def test_corrupt_primary_recovers_last_complete_save(self):
        data = {'ws/agent': {'pane_id': 'p', 'tab_id': 't', 'sessions': {}}}
        bridge_state.save_map(data)
        Path(bridge_state.MAP_PATH).write_text('{broken')
        self.assertEqual(bridge_state.load_map(), data)
        bridge_state.save_map(data)
        self.assertEqual(json.loads(Path(bridge_state.MAP_PATH).read_text()), data)

    def test_unrecoverable_map_retains_spool_until_repaired(self):
        map_path = Path(bridge_state.MAP_PATH)
        map_path.write_text('{broken')
        event = {'event': 'turn.start', 'session_type': 'user', 'agent_name': 'agent',
                 'session_id': 'sid', 'workspace_id': 'ws'}
        payload = self.spool / 'event.json'
        payload.write_text(json.dumps(event))
        self.assertEqual(bridge.drain_spool(), 0)
        self.assertEqual(json.loads(payload.read_text()), event)
        self.assertEqual(map_path.read_text(), '{broken')
        bridge_state.save_map({'ws/agent': {'pane_id': 'p', 'tab_id': 't', 'sessions': {}}})
        with patch.object(bridge_state, 'herdr', return_value={'result': {}}), patch.object(bridge, 'herdr'):
            self.assertEqual(bridge.drain_spool(), 1)
        self.assertFalse(payload.exists())
        self.assertEqual(bridge_state.load_map()['ws/agent']['sessions']['sid']['state'], 'working')

    def test_reconcile_telemetry_does_not_hold_the_map_lock(self):
        import fcntl
        data = {'loop/ws/loop': {'kind': 'loop', 'pane_id': 'p', 'tab_id': 't', 'loop': 'loop',
                                'run_id': 'run1', 'sessions': {'run1': {'state': 'idle'}}}}
        bridge_state.save_map(data)
        methods = []

        def rpc(method, params):
            methods.append(method)
            with (Path(bridge_state.STATE_DIR) / '.lock').open('w') as lock:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                fcntl.flock(lock, fcntl.LOCK_UN)
            return {'result': {}}

        with patch.object(bridge_loops, 'query_loop_status', return_value='running'), \
                patch.object(bridge_loops, 'herdr', side_effect=rpc):
            self.assertEqual(bridge_loops.reconcile_loops(), [('loop/ws/loop', 'run1', 'running')])
        self.assertEqual(methods, ['pane.report_agent', 'pane.report_metadata'])
        self.assertEqual(bridge_state.load_map()['loop/ws/loop']['sessions']['run1']['state'], 'working')

    def test_state_and_existing_files_are_private_under_permissive_umask(self):
        state = Path(self.temp.name) / 'herdr-bridge'
        state.mkdir(mode=0o755)
        for name in ('panes.json', 'panes.json.tmp', 'panes.json.bak.tmp', 'bridge.log', '.lock', '.watch-lock'):
            path = state / name
            path.write_text('{}')
            path.chmod(0o666)
        root = Path(__file__).resolve().parents[1]
        result = subprocess.run([sys.executable, '-B', '-c',
                                 'import bridge_state as s; import bridge_loops as l; '
                                 's.save_map({}); s.log("private"); '
                                 'lock=s.Locked(); lock.__enter__(); lock.__exit__(); l.watch_loops()'],
                                env={**os.environ, 'PYTHONPATH': str(root), 'XDG_STATE_HOME': self.temp.name},
                                capture_output=True, umask=0o022)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(state.stat().st_mode & 0o777, 0o700)
        for path in state.iterdir():
            self.assertEqual(path.stat().st_mode & 0o777, 0o600, path.name)


if __name__ == '__main__':
    unittest.main()
