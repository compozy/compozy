"""A local receiving-API simulation; no network calls or third-party packages."""

from contextlib import contextmanager
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
from tempfile import TemporaryDirectory


CRASH_EXIT = 23
OPERATION = "review-42/comment-1"
BODY = "Please add a timeout to this request."


@contextmanager
def transaction(path):
    db = sqlite3.connect(path)
    try:
        with db:
            yield db
    finally:
        db.close()


def initialize(root):
    with transaction(root / "caller.db") as db:
        db.execute("CREATE TABLE operations (id TEXT PRIMARY KEY, body TEXT, receipt INTEGER)")
        db.execute("INSERT INTO operations VALUES (?, ?, NULL)", (OPERATION, BODY))
    with transaction(root / "receiver.db") as db:
        db.execute("CREATE TABLE comments (id INTEGER PRIMARY KEY, key TEXT UNIQUE, body TEXT)")


def publish(root, key, body):
    # The receiver owns both the effect and deduplication in one transaction.
    with transaction(root / "receiver.db") as db:
        db.execute("BEGIN IMMEDIATE")
        prior = db.execute("SELECT id, body FROM comments WHERE key = ?", (key,)).fetchone()
        if prior:
            if prior[1] != body:
                raise ValueError("same key, different body")
            return prior[0]
        cursor = db.execute("INSERT INTO comments (key, body) VALUES (?, ?)", (key, body))
        comment_id = cursor.lastrowid
    # Leaving the context above commits before the receipt reaches the caller.
    return comment_id


def attempt(root, mode, crash):
    with transaction(root / "caller.db") as db:
        operation_id, body, receipt = db.execute("SELECT * FROM operations").fetchone()
    if receipt is not None:
        return
    key = operation_id if mode == "idempotent" else None
    receipt = publish(root, key, body)
    if crash:
        os._exit(CRASH_EXIT)  # Deliberately terminate without caller cleanup.
    with transaction(root / "caller.db") as db:
        db.execute("UPDATE operations SET receipt = ? WHERE id = ?", (receipt, operation_id))


def observed(root):
    with transaction(root / "receiver.db") as db:
        comments = db.execute("SELECT count(*) FROM comments").fetchone()[0]
    with transaction(root / "caller.db") as db:
        receipt = db.execute("SELECT receipt FROM operations").fetchone()[0]
    return comments, receipt


def run_worker(root, mode, crash=False):
    result = subprocess.run(
        [sys.executable, str(Path(__file__).resolve()), "worker", str(root), mode,
         "crash" if crash else "finish"],
        check=False,
    )
    expected = CRASH_EXIT if crash else 0
    if result.returncode != expected:
        raise RuntimeError(f"worker exited {result.returncode}; expected {expected}")


def demo():
    for mode in ("naive", "idempotent"):
        with TemporaryDirectory(prefix="retry-lab-") as directory:
            root = Path(directory)
            initialize(root)
            run_worker(root, mode, crash=True)
            comments, receipt = observed(root)
            assert (comments, receipt) == (1, None)
            print(f"{mode}: after crash -> comments={comments}, local_receipt={receipt}")

            run_worker(root, mode)
            comments, receipt = observed(root)
            expected = 2 if mode == "naive" else 1
            assert (comments, receipt) == (expected, expected)
            print(f"{mode}: after retry -> comments={comments}, local_receipt={receipt}")

            run_worker(root, mode)
            assert observed(root) == (comments, receipt)
            print(f"{mode}: after another run -> comments={comments}, local_receipt={receipt}")
            if mode == "idempotent":
                try:
                    publish(root, OPERATION, "A different comment.")
                except ValueError as error:
                    print(f"{mode}: changed payload rejected -> {error}")
                else:
                    raise AssertionError("receiver accepted changed content with the same key")
                assert observed(root) == (1, 1)
    print("All assertions passed. Temporary databases removed.")


if __name__ == "__main__":
    if len(sys.argv) == 1:
        demo()
    elif len(sys.argv) == 5 and sys.argv[1] == "worker":
        attempt(Path(sys.argv[2]), sys.argv[3], sys.argv[4] == "crash")
    else:
        raise SystemExit("Run without arguments: python3 retry_lab.py")
