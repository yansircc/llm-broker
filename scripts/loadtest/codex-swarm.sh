#!/usr/bin/env bash
# 80 路真实 codex CLI 并发压测。打的是 ~/.codex 配置里的线上 broker (ccc.210k.cc)。
# 用法: ./codex-swarm.sh [N] [PROMPT]
#   N      并发数, 默认 80
#   PROMPT 每个任务的提示词, 默认一句短指令
set -u

N="${1:-80}"
PROMPT="${2:-Reply with exactly one short sentence: report your status as OK.}"
MODEL="${MODEL:-gpt-5.5}"
RUN_ID="$(date +%Y%m%d-%H%M%S)"
OUT="/tmp/codex-swarm/$RUN_ID"
mkdir -p "$OUT"

now_ms() { python3 -c 'import time;print(int(time.time()*1000))'; }

echo "run_id=$RUN_ID  N=$N  model=$MODEL  out=$OUT"
START_S=$(date +%s)
echo "window_start_epoch=$START_S"

run_job() {
  local i="$1" t0 t1 rc
  t0=$(now_ms)
  codex exec -s read-only --skip-git-repo-check --ephemeral \
    -m "$MODEL" -C "$OUT" "$PROMPT (job #$i)" \
    >"$OUT/job-$i.out" 2>"$OUT/job-$i.err"
  rc=$?
  t1=$(now_ms)
  echo "$i,$rc,$((t1 - t0))" >>"$OUT/results.csv"
}

echo "launching $N concurrent codex exec ..."
for i in $(seq 1 "$N"); do
  run_job "$i" &
done
wait
END_S=$(date +%s)
echo "window_end_epoch=$END_S  elapsed_wall=$((END_S - START_S))s"

# ---- summary ----
echo "=================== SUMMARY ==================="
TOTAL=$(wc -l <"$OUT/results.csv" | tr -d ' ')
OK=$(awk -F, '$2==0' "$OUT/results.csv" | wc -l | tr -d ' ')
FAIL=$((TOTAL - OK))
printf "total=%s  ok=%s  fail=%s  success_rate=%s%%\n" "$TOTAL" "$OK" "$FAIL" \
  "$(awk -v o="$OK" -v t="$TOTAL" 'BEGIN{printf (t?"%.1f":"0"), (t?o*100.0/t:0)}')"
# duration percentiles via sort
D=()
while IFS= read -r line; do D+=("$line"); done < <(cut -d, -f3 "$OUT/results.csv" | sort -n)
if [ "${#D[@]}" -gt 0 ]; then
  n=${#D[@]}
  printf "per-job wall_ms: min=%s  p50=%s  p90=%s  max=%s\n" \
    "${D[0]}" "${D[$((n*50/100))]}" "${D[$((n*90/100))]}" "${D[$((n-1))]}"
fi
if [ "$FAIL" -gt 0 ]; then
  echo "failed jobs (job:exitcode):"
  awk -F, '$2!=0{printf "  #%s exit=%s\n",$1,$2}' "$OUT/results.csv"
  echo "stderr tail of first failures:"
  awk -F, '$2!=0{print $1}' "$OUT/results.csv" | head -5 | while read -r j; do
    echo "  --- job #$j ---"; tail -3 "$OUT/job-$j.err"
  done
fi
echo "window for analyze.sh:  ./analyze.sh $START_S $END_S"
