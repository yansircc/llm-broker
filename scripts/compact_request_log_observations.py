#!/usr/bin/env python3
"""Migrate historical request_log observation payload into per-request JSON files.

Read-only on the SQLite database. For every row in `request_log` that has any
observation payload (headers/meta/body excerpts, upstream URL/request id,
session metadata, etc.), write a per-row file at:

    <log-dir>/YYYY/MM/DD/<id>.json

The file shape mirrors the Go writer in internal/requestlog/artifact.go so that
existing rows look the same as new rows produced by the P1 binary with
LOG_BLOBS=true.

This script does NOT modify the database. It is safe to run while the broker
service is live (SQLite WAL mode allows concurrent readers and writers).

It is idempotent: re-running on the same DB and log-dir is a no-op for rows
whose file already exists.

Run:
    python3 scripts/compact_request_log_observations.py \\
        --db /path/to/data.db \\
        --log-dir /path/to/data-dir/request-log-blobs
"""

import argparse
import json
import os
import sqlite3
import sys
import time
from pathlib import Path
from typing import Any


SCHEMA = "llm-broker.request_log.v2"

# All columns we expect in the legacy 37-column schema. Missing columns are
# tolerated (rows from intermediate migrations may not have all of them).
ALL_COLUMNS = [
    "id",
    "user_id",
    "account_id",
    "provider",
    "surface",
    "model",
    "path",
    "cell_id",
    "bucket_key",
    "session_uuid",
    "binding_source",
    "client_headers_json",
    "client_body_excerpt",
    "request_meta_json",
    "input_tokens",
    "output_tokens",
    "cache_read_tokens",
    "cache_create_tokens",
    "cost_usd",
    "status",
    "effect_kind",
    "upstream_status",
    "upstream_url",
    "upstream_request_headers_json",
    "upstream_request_meta_json",
    "upstream_request_body_excerpt",
    "upstream_request_id",
    "upstream_headers_json",
    "upstream_response_meta_json",
    "upstream_response_body_excerpt",
    "upstream_error_type",
    "upstream_error_message",
    "request_bytes",
    "attempt_count",
    "duration_ms",
    "created_at",
]


def available_columns(con: sqlite3.Connection) -> list[str]:
    cur = con.execute("PRAGMA table_info(request_log)")
    return [
        (row[1].decode("utf-8", errors="replace") if isinstance(row[1], bytes) else row[1])
        for row in cur.fetchall()
    ]


def json_value(raw: str | None) -> Any:
    if raw is None:
        return None
    raw = raw.strip()
    if not raw or raw in ("null", "{}"):
        return None
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return raw


def compact_map(values: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, value in values.items():
        if value is None:
            continue
        if isinstance(value, str) and not value.strip():
            continue
        if isinstance(value, (list, dict)) and not value:
            continue
        out[key] = value
    return out


def build_payload(row: dict[str, Any]) -> dict[str, Any]:
    created_at_unix = row.get("created_at") or 0
    if isinstance(created_at_unix, str):
        try:
            created_at_unix = int(created_at_unix)
        except ValueError:
            created_at_unix = 0
    created_at_iso = (
        time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(created_at_unix))
        if created_at_unix
        else ""
    )

    facts = compact_map(
        {
            "user_id": row.get("user_id"),
            "account_id": row.get("account_id"),
            "provider": row.get("provider"),
            "surface": row.get("surface"),
            "model": row.get("model"),
            "cell_id": row.get("cell_id"),
            "status": row.get("status"),
            "effect_kind": row.get("effect_kind"),
            "upstream_status": row.get("upstream_status"),
            "upstream_error_type": row.get("upstream_error_type"),
            "input_tokens": row.get("input_tokens"),
            "output_tokens": row.get("output_tokens"),
            "cache_read_tokens": row.get("cache_read_tokens"),
            "cache_create_tokens": row.get("cache_create_tokens"),
            "cost_usd": row.get("cost_usd"),
            "duration_ms": row.get("duration_ms"),
            "path": row.get("path"),
            "bucket_key": row.get("bucket_key"),
            "session_uuid": row.get("session_uuid"),
            "binding_source": row.get("binding_source"),
            "upstream_url": row.get("upstream_url"),
            "upstream_request_id": row.get("upstream_request_id"),
            "upstream_error_message": row.get("upstream_error_message"),
            "request_bytes": row.get("request_bytes"),
            "attempt_count": row.get("attempt_count"),
        }
    )

    payload: dict[str, Any] = {
        "schema": SCHEMA,
        "id": row.get("id"),
        "created_at": created_at_iso,
        "facts": facts,
    }

    client = compact_map(
        {
            "headers": json_value(row.get("client_headers_json")),
            "meta": json_value(row.get("request_meta_json")),
            "body_excerpt": row.get("client_body_excerpt"),
        }
    )
    if client:
        payload["client"] = client

    upstream_request = compact_map(
        {
            "url": row.get("upstream_url"),
            "headers": json_value(row.get("upstream_request_headers_json")),
            "meta": json_value(row.get("upstream_request_meta_json")),
            "body_excerpt": row.get("upstream_request_body_excerpt"),
        }
    )
    if upstream_request:
        payload["upstream_request"] = upstream_request

    upstream_response = compact_map(
        {
            "headers": json_value(row.get("upstream_headers_json")),
            "meta": json_value(row.get("upstream_response_meta_json")),
            "body_excerpt": row.get("upstream_response_body_excerpt"),
            "error_type": row.get("upstream_error_type"),
            "error_message": row.get("upstream_error_message"),
        }
    )
    if upstream_response:
        payload["upstream_response"] = upstream_response

    return payload


def has_observation(row: dict[str, Any]) -> bool:
    keys = (
        "client_headers_json",
        "client_body_excerpt",
        "request_meta_json",
        "upstream_request_headers_json",
        "upstream_request_meta_json",
        "upstream_request_body_excerpt",
        "upstream_headers_json",
        "upstream_response_meta_json",
        "upstream_response_body_excerpt",
        "upstream_url",
        "upstream_request_id",
        "upstream_error_message",
        "path",
        "bucket_key",
        "session_uuid",
        "binding_source",
        "request_bytes",
        "attempt_count",
    )
    for key in keys:
        value = row.get(key)
        if value is None:
            continue
        if isinstance(value, str) and value.strip() in ("", "{}", "null"):
            continue
        if isinstance(value, int) and value == 0:
            continue
        return True
    return False


def day_dir(log_dir: Path, created_at_unix: int) -> Path:
    return log_dir / time.strftime("%Y/%m/%d", time.gmtime(created_at_unix))


def file_path(log_dir: Path, row_id: int, created_at_unix: int) -> Path:
    return day_dir(log_dir, created_at_unix) / f"{row_id}.json"


def migrate(
    db_path: Path,
    log_dir: Path,
    *,
    limit: int | None,
    batch_size: int,
    dry_run: bool,
) -> dict[str, int]:
    if not db_path.exists():
        raise FileNotFoundError(f"db not found: {db_path}")

    uri = f"file:{db_path.resolve()}?mode=ro"
    con = sqlite3.connect(uri, uri=True)
    # Historical excerpts may contain bytes that aren't valid UTF-8 (LLM
    # responses truncated mid-codepoint). text_factory=bytes returns raw bytes
    # and the build_payload helpers decode with errors="replace" downstream.
    con.text_factory = bytes
    try:
        con.row_factory = sqlite3.Row
        present = set(available_columns(con))
        cols = [c for c in ALL_COLUMNS if c in present]
        if "id" not in cols or "created_at" not in cols:
            raise RuntimeError(
                f"request_log table is missing id/created_at columns: {sorted(present)}"
            )

        order = "ORDER BY id"
        if limit is not None:
            order += f" LIMIT {int(limit)}"

        select_sql = f"SELECT {', '.join(cols)} FROM request_log {order}"
        cur = con.execute(select_sql)

        stats = {
            "rows_seen": 0,
            "rows_skipped_empty": 0,
            "rows_skipped_existing": 0,
            "files_written": 0,
            "files_failed": 0,
        }

        log_dir = log_dir.resolve()

        while True:
            rows = cur.fetchmany(batch_size)
            if not rows:
                break
            for row_obj in rows:
                row = {
                    k: (
                        v.decode("utf-8", errors="replace") if isinstance(v, bytes) else v
                    )
                    for k, v in dict(row_obj).items()
                }
                stats["rows_seen"] += 1
                if not has_observation(row):
                    stats["rows_skipped_empty"] += 1
                    continue
                row_id = int(row["id"])
                created_at = int(row.get("created_at") or 0)
                target = file_path(log_dir, row_id, created_at)
                if target.exists():
                    stats["rows_skipped_existing"] += 1
                    continue
                payload = build_payload(row)
                if dry_run:
                    stats["files_written"] += 1
                    continue
                try:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with target.open("w", encoding="utf-8") as fh:
                        json.dump(payload, fh, indent=2, ensure_ascii=False)
                    stats["files_written"] += 1
                except OSError as exc:
                    stats["files_failed"] += 1
                    print(f"[warn] write {target}: {exc}", file=sys.stderr)
        return stats
    finally:
        con.close()


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True, type=Path, help="path to SQLite DB")
    parser.add_argument(
        "--log-dir",
        required=True,
        type=Path,
        help="output directory (typically <data-dir>/request-log-blobs)",
    )
    parser.add_argument("--limit", type=int, default=None, help="max rows to process")
    parser.add_argument("--batch", type=int, default=500, help="fetch batch size")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="count what would be written without touching disk",
    )
    args = parser.parse_args(argv)

    stats = migrate(
        args.db,
        args.log_dir,
        limit=args.limit,
        batch_size=args.batch,
        dry_run=args.dry_run,
    )

    if args.dry_run:
        print("dry-run summary:")
    else:
        print("migration summary:")
    for key, value in stats.items():
        print(f"  {key:24s} {value}")
    return 0 if stats["files_failed"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
