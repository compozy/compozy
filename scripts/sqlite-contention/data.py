"""Synthetic retained data, lossless-upgrade proofs, and index receipts."""
from datetime import datetime, timedelta, timezone
import hashlib
import json
import sqlite3
import time
from pathlib import Path


def connect(path):
    db = sqlite3.connect(path)
    db.execute("PRAGMA foreign_keys=ON")
    return db


def seed_large(root, target_bytes):
    path = Path(root) / "compozy.db"
    with connect(path) as db:
        # One released schema, no migration edits and no disabled constraints.
        columns = [r[1] for r in db.execute("PRAGMA table_info(event_summaries)")]
        template = dict(zip(columns, db.execute("SELECT * FROM event_summaries LIMIT 1").fetchone()))
        sql = "INSERT INTO event_summaries (" + ",".join(columns) + ") VALUES (" + ",".join("?" for _ in columns) + ")"
        next_seq = db.execute("SELECT MAX(seq)+1 FROM event_summaries").fetchone()[0]
        page_size = db.execute("PRAGMA page_size").fetchone()[0]
        started = time.monotonic()
        while db.execute("PRAGMA page_count").fetchone()[0] * page_size < target_bytes:
            batch = []
            for seq in range(next_seq, next_seq + 5000):
                row = dict(template)
                row.update(seq=seq, id=f"history-{seq:09d}", session_id=f"contention-session-{seq % 30:02d}",
                           workspace_id="contention-workspace", agent_name="coder", provider="claude",
                           type="agent_message" if seq % 4 else "tool_call", outcome="failure" if seq % 10 == 0 else "info",
                           content_json=json.dumps({"message": "synthetic " * 26, "ordinal": seq}, separators=(",", ":")),
                           summary=f"Retained synthetic observation {seq}",
                           timestamp=(datetime(2026, 10, 26, tzinfo=timezone.utc) + timedelta(seconds=seq % (7*86400))).isoformat().replace("+00:00", "Z"),
                           task_id=f"task-{seq % 1000}", run_id=f"run-{seq % 2000}", workflow_id=f"workflow-{seq % 10}",
                           worktree_id=f"worktree-{seq % 50}", claim_token_hash=f"synthetic-hash-{seq % 400:04d}",
                           lease_until="2026-11-02T12:00:00.000000000Z", coordinator_session_id="contention-session-00",
                           scheduler_reason="synthetic", hook_event="after_tool", hook_name="synthetic-hook",
                           actor_kind="session", actor_id=f"contention-session-{seq % 30:02d}", release_reason="completed",
                           parent_session_id="contention-session-00" if seq % 3 else "", root_session_id="contention-session-00",
                           spawn_depth=1 if seq % 3 else 0)
                batch.append(tuple(row[c] for c in columns))
            db.executemany(sql, batch)
            db.commit()
            next_seq += len(batch)
        # Keep a retired high-water mark above the last surviving row.
        db.execute("DELETE FROM event_summaries WHERE seq > ?", (next_seq - 11,))
        db.commit()
        db.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        return {"seconds": time.monotonic() - started, "logical_bytes": db.execute("PRAGMA page_count").fetchone()[0] * page_size,
                "rows": db.execute("SELECT COUNT(*) FROM event_summaries").fetchone()[0]}


def ledger_path(root):
    core = Path(root) / "compozy.db"
    aux = Path(str(core) + ".telemetry.db")
    return aux if aux.exists() else core


def inspect_data(root):
    path = ledger_path(root)
    with connect(path) as db:
        columns = sorted(r[1] for r in db.execute("PRAGMA table_info(event_summaries)"))
        digest = hashlib.sha256()
        digest.update(json.dumps(columns, separators=(",", ":")).encode())
        count = 0
        for row in db.execute("SELECT " + ",".join(columns) + " FROM event_summaries ORDER BY seq"):
            digest.update(json.dumps(row, separators=(",", ":"), ensure_ascii=True).encode() + b"\n")
            count += 1
        integrity = db.execute("PRAGMA integrity_check").fetchall()
        if integrity != [("ok",)]:
            raise RuntimeError(f"integrity failed: {integrity}")
        indexes = [dict(zip(("name", "sql"), r)) for r in db.execute("SELECT name,sql FROM sqlite_schema WHERE type='index' AND tbl_name='event_summaries' ORDER BY name")]
        pages = {r[0]: {"pages": r[1], "bytes": r[2]} for r in db.execute("SELECT name,COUNT(*),SUM(pgsize) FROM dbstat WHERE name='event_summaries' OR name LIKE '%summaries%' GROUP BY name")}
        queries = {
            "profile_recent": ("SELECT seq,id FROM event_summaries WHERE profile_id=? ORDER BY timestamp DESC,seq DESC LIMIT 100", (db.execute("SELECT profile_id FROM event_summaries LIMIT 1").fetchone()[0],)),
            "workspace_replay": ("SELECT seq,id FROM event_summaries WHERE workspace_id=? AND seq>? ORDER BY seq LIMIT 100", ("contention-workspace", 100000)),
            "session_recent": ("SELECT seq,id FROM event_summaries WHERE session_id=? ORDER BY timestamp DESC,seq DESC LIMIT 100", ("contention-session-07",)),
            "provider_recent": ("SELECT seq,id FROM event_summaries WHERE provider=? ORDER BY timestamp DESC,seq DESC LIMIT 100", ("claude",)),
            "failures": ("SELECT seq,id FROM event_summaries WHERE outcome='failure' ORDER BY timestamp DESC,seq DESC LIMIT 100", ()),
            "task": ("SELECT seq,id FROM event_summaries WHERE task_id=? ORDER BY timestamp DESC,seq DESC LIMIT 100", ("task-17",)),
            "run": ("SELECT seq,id FROM event_summaries WHERE run_id=? ORDER BY timestamp DESC,seq DESC LIMIT 100", ("run-23",)),
        }
        plans = {name: {"sql": sql, "bindings": args, "plan": db.execute("EXPLAIN QUERY PLAN " + sql, args).fetchall()} for name, (sql, args) in queries.items()}
        return {"all_columns": columns, "sha256": digest.hexdigest(), "rows": count,
                "max_seq": db.execute("SELECT MAX(seq) FROM event_summaries").fetchone()[0],
                "high_water": db.execute("SELECT seq FROM sqlite_sequence WHERE name='event_summaries'").fetchone()[0],
                "integrity": "ok", "indexes": indexes, "pages": pages, "plans": plans}
