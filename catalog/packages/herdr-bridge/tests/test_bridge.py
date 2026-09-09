"""Behavioral regressions for the catalog bridge's Python runtime.

Owning layer: extension hook consumer, row store, and event renderer.
Only socket/HTTP I/O is mocked; state, files, rendering, and hook execution are real.
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

    def test_hook_spool_is_private_and_xdg_drainer_consumes_it(self):
        root = Path(__file__).resolve().parents[1]
        xdg = Path(self.temp.name) / 'xdg'
        spool = xdg / 'herdr-bridge' / 'spool'
        spool.mkdir(parents=True, mode=0o755)
        spool.chmod(0o755)
        env = {**os.environ, 'XDG_STATE_HOME': str(xdg), 'HOME': self.temp.name,
               'PYTHONDONTWRITEBYTECODE': '1'}
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
        while list(spool.glob('*.json')) and time.monotonic() < deadline:
            time.sleep(0.05)
        self.assertEqual(list(spool.glob('*.json')), [])

    def test_corrupt_state_is_preserved_and_command_failure_is_logged(self):
        state = Path(self.temp.name) / 'herdr-bridge'
        state.mkdir()
        map_path = state / 'panes.json'
        map_path.write_text('{broken')
        command = Path(__file__).resolve().parents[1] / 'bridge.py'
        result = subprocess.run([sys.executable, '-B', str(command), '--status'],
                                env={**os.environ, 'XDG_STATE_HOME': self.temp.name}, capture_output=True)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(map_path.read_text(), '{broken')
        self.assertIn('unexpected bridge failure:', (state / 'bridge.log').read_text())


if __name__ == '__main__':
    unittest.main()
