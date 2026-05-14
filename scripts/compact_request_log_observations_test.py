"""Tests for compact_request_log_observations.py.

Run with:
    python3 -m pytest scripts/compact_request_log_observations_test.py -v
or:
    python3 scripts/compact_request_log_observations_test.py
"""

import json
import sqlite3
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "compact_request_log_observations.py"


LEGACY_SCHEMA = """
CREATE TABLE request_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    surface TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    cell_id TEXT NOT NULL DEFAULT '',
    bucket_key TEXT NOT NULL DEFAULT '',
    session_uuid TEXT NOT NULL DEFAULT '',
    binding_source TEXT NOT NULL DEFAULT '',
    client_headers_json TEXT NOT NULL DEFAULT '{}',
    client_body_excerpt TEXT NOT NULL DEFAULT '',
    request_meta_json TEXT NOT NULL DEFAULT '{}',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_create_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    effect_kind TEXT NOT NULL DEFAULT '',
    upstream_status INTEGER NOT NULL DEFAULT 0,
    upstream_url TEXT NOT NULL DEFAULT '',
    upstream_request_headers_json TEXT NOT NULL DEFAULT '{}',
    upstream_request_meta_json TEXT NOT NULL DEFAULT '{}',
    upstream_request_body_excerpt TEXT NOT NULL DEFAULT '',
    upstream_request_id TEXT NOT NULL DEFAULT '',
    upstream_headers_json TEXT NOT NULL DEFAULT '{}',
    upstream_response_meta_json TEXT NOT NULL DEFAULT '{}',
    upstream_response_body_excerpt TEXT NOT NULL DEFAULT '',
    upstream_error_type TEXT NOT NULL DEFAULT '',
    upstream_error_message TEXT NOT NULL DEFAULT '',
    request_bytes INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);
"""


def _seed_legacy_db(path: Path, rows: list[dict]) -> None:
    con = sqlite3.connect(path)
    try:
        con.executescript(LEGACY_SCHEMA)
        for row in rows:
            cols = ", ".join(row.keys())
            placeholders = ", ".join("?" for _ in row)
            con.execute(
                f"INSERT INTO request_log ({cols}) VALUES ({placeholders})",
                list(row.values()),
            )
        con.commit()
    finally:
        con.close()


def _run_script(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        check=False,
        capture_output=True,
        text=True,
    )


class CompactRequestLogTest(unittest.TestCase):
    def test_writes_file_per_row_with_observation_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            db_path = tmp_path / "data.db"
            log_dir = tmp_path / "request-log-blobs"

            created_at = int(time.time())
            _seed_legacy_db(
                db_path,
                [
                    {
                        "user_id": "u-1",
                        "account_id": "acct-1",
                        "provider": "claude",
                        "surface": "compat",
                        "model": "claude-sonnet-4-6",
                        "path": "/compat/v1/chat/completions",
                        "cell_id": "cell-compat-1",
                        "bucket_key": "claude:bucket-1",
                        "session_uuid": "sess-1",
                        "binding_source": "session_bound",
                        "client_headers_json": '{"Content-Type":"application/json"}',
                        "client_body_excerpt": '{"messages":[{"role":"user","content":"hi"}]}',
                        "request_meta_json": '{"stream":false}',
                        "input_tokens": 10,
                        "output_tokens": 5,
                        "cost_usd": 0.01,
                        "status": "upstream_400",
                        "effect_kind": "cooldown",
                        "upstream_status": 400,
                        "upstream_url": "https://api.anthropic.com/v1/messages",
                        "upstream_request_headers_json": '{"anthropic-version":"2023-06-01"}',
                        "upstream_request_meta_json": '{"method":"POST"}',
                        "upstream_request_body_excerpt": '{"model":"claude-sonnet-4-6"}',
                        "upstream_request_id": "req-abc",
                        "upstream_headers_json": '{"request-id":"req-abc"}',
                        "upstream_response_meta_json": '{"status":400}',
                        "upstream_response_body_excerpt": '{"error":{"message":"bad"}}',
                        "upstream_error_type": "invalid_request_error",
                        "upstream_error_message": "bad",
                        "request_bytes": 256,
                        "attempt_count": 1,
                        "duration_ms": 800,
                        "created_at": created_at,
                    },
                    # An "ok" row with no observation payload should be skipped.
                    {
                        "user_id": "u-1",
                        "account_id": "acct-1",
                        "model": "claude-sonnet-4-6",
                        "status": "ok",
                        "created_at": created_at,
                    },
                ],
            )

            result = _run_script("--db", str(db_path), "--log-dir", str(log_dir))
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertIn("rows_seen                2", result.stdout)
            self.assertIn("files_written            1", result.stdout)
            self.assertIn("rows_skipped_empty       1", result.stdout)

            day = time.strftime("%Y/%m/%d", time.gmtime(created_at))
            target = log_dir / day / "1.json"
            self.assertTrue(target.exists(), f"expected {target} to exist")

            payload = json.loads(target.read_text())
            self.assertEqual(payload["schema"], "llm-broker.request_log.v2")
            self.assertEqual(payload["id"], 1)
            facts = payload["facts"]
            self.assertEqual(facts["upstream_error_type"], "invalid_request_error")
            self.assertEqual(facts["upstream_error_message"], "bad")
            self.assertEqual(facts["session_uuid"], "sess-1")
            self.assertEqual(facts["path"], "/compat/v1/chat/completions")
            self.assertEqual(facts["cell_id"], "cell-compat-1")
            self.assertEqual(
                payload["client"]["body_excerpt"],
                '{"messages":[{"role":"user","content":"hi"}]}',
            )
            self.assertEqual(
                payload["upstream_response"]["body_excerpt"],
                '{"error":{"message":"bad"}}',
            )
            self.assertEqual(
                payload["upstream_response"]["error_type"], "invalid_request_error"
            )

    def test_is_idempotent_when_files_already_exist(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            db_path = tmp_path / "data.db"
            log_dir = tmp_path / "request-log-blobs"

            created_at = int(time.time())
            _seed_legacy_db(
                db_path,
                [
                    {
                        "user_id": "u-1",
                        "account_id": "acct-1",
                        "model": "claude-sonnet-4-6",
                        "status": "upstream_400",
                        "client_body_excerpt": "first body",
                        "created_at": created_at,
                    },
                ],
            )

            first = _run_script("--db", str(db_path), "--log-dir", str(log_dir))
            self.assertEqual(first.returncode, 0, msg=first.stderr)

            day = time.strftime("%Y/%m/%d", time.gmtime(created_at))
            target = log_dir / day / "1.json"
            self.assertTrue(target.exists())
            mtime_first = target.stat().st_mtime

            time.sleep(1.0)
            second = _run_script("--db", str(db_path), "--log-dir", str(log_dir))
            self.assertEqual(second.returncode, 0, msg=second.stderr)
            self.assertIn("rows_skipped_existing    1", second.stdout)
            self.assertIn("files_written            0", second.stdout)

            mtime_second = target.stat().st_mtime
            self.assertEqual(
                mtime_first,
                mtime_second,
                "second run must not rewrite existing files",
            )

    def test_does_not_modify_database(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            db_path = tmp_path / "data.db"
            log_dir = tmp_path / "request-log-blobs"
            _seed_legacy_db(
                db_path,
                [
                    {
                        "user_id": "u-1",
                        "account_id": "acct-1",
                        "model": "claude-sonnet-4-6",
                        "status": "upstream_400",
                        "client_body_excerpt": "first body",
                        "created_at": int(time.time()),
                    },
                ],
            )
            before = db_path.read_bytes()

            result = _run_script("--db", str(db_path), "--log-dir", str(log_dir))
            self.assertEqual(result.returncode, 0, msg=result.stderr)

            after = db_path.read_bytes()
            self.assertEqual(before, after, "script must not modify the database file")

    def test_dry_run_writes_nothing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            db_path = tmp_path / "data.db"
            log_dir = tmp_path / "request-log-blobs"
            _seed_legacy_db(
                db_path,
                [
                    {
                        "user_id": "u-1",
                        "account_id": "acct-1",
                        "model": "claude-sonnet-4-6",
                        "status": "upstream_400",
                        "client_body_excerpt": "first body",
                        "created_at": int(time.time()),
                    },
                ],
            )

            result = _run_script(
                "--db", str(db_path), "--log-dir", str(log_dir), "--dry-run"
            )
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertIn("dry-run summary", result.stdout)
            self.assertFalse(
                log_dir.exists(), "dry-run must not create the output directory"
            )


if __name__ == "__main__":
    unittest.main()
