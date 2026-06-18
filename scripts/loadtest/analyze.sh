#!/usr/bin/env bash
# 压测后归因: 在窗口 [START END] 内对线上 request_log 做分解。
# 用法: ./analyze.sh START_EPOCH END_EPOCH
set -u
START="${1:?need start epoch}"
END="${2:?need end epoch}"
REMOTE="${REMOTE:-root@172.236.22.238}"
PAD=30  # 窗口两端各放宽 30s, 兜住尾部请求

ssh -o ConnectTimeout=8 "$REMOTE" "sqlite3 -header -column /var/lib/llm-broker/llm-broker.db \"
  SELECT '== by outcome ==' AS section;
  SELECT status, upstream_status us, effect_kind, count(*) n, round(avg(duration_ms)) avg_ms, max(duration_ms) max_ms
    FROM request_log
   WHERE provider='codex' AND created_at BETWEEN $((START-PAD)) AND $((END+PAD))
   GROUP BY status, upstream_status, effect_kind ORDER BY n DESC;
  SELECT '== per account (spread) ==' AS section;
  SELECT substr(account_id,1,8) acct, count(*) n,
         sum(case when upstream_status>=400 then 1 else 0 end) errs,
         round(avg(duration_ms)) avg_ms
    FROM request_log
   WHERE provider='codex' AND created_at BETWEEN $((START-PAD)) AND $((END+PAD))
   GROUP BY account_id ORDER BY n DESC;
\""
