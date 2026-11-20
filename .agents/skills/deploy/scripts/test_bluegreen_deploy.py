"""Exercise deployment ordering and failure recovery without SSH or services."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("bluegreen_deploy.sh").resolve()
REPO = SCRIPT.parents[4]
MOCK_COMMON = r'''
REPO_ROOT="$CASE_DIR"
REMOTE=fixture
TMP_REMOTE=/fixture/candidate
mark() { printf '%s\n' "$1" >> "$TRACE"; [[ "${FAIL_STAGE:-}" != "$1" ]]; }
load_bluegreen_layout() {
    BLUE_SLOT=blue; GREEN_SLOT=green; ACTIVE_SLOT=green
    BLUE_SERVICE=llm-broker-blue; GREEN_SERVICE=llm-broker-green
    BLUE_PORT=3001; GREEN_PORT=3002; BLUE_BIN=/fixture/blue; GREEN_BIN=/fixture/green
}
build_release_artifact() { mark build; }
create_snapshot() { mark snapshot; printf 'snapshot-test\n'; }
upload_candidate_binary() { mark upload; }
run_uploaded_binary_migrate() { mark migrate; }
wait_for_remote_local_health() { mark candidate_health; }
set_bluegreen_active_slot() { mark switch; }
remote_reload_caddy() { mark reload; }
wait_for_site_health() { mark public_health; }
verify_db_invariants() { mark verify; }
show_recent_restart_events() { mark events; }
run_nonfatal_smoke_suite() { mark smoke; }
restore_snapshot() { mark restore; }
ssh() {
    local payload=""
    if [[ "$*" == *'bash -s'* ]]; then payload="$(cat)"; fi
    if [[ "$payload" == *'systemctl stop "$ACTIVE_SERVICE_NAME"'* ]]; then mark drain
    elif [[ "$payload" == *'systemctl restart "$INACTIVE_SERVICE_NAME"'* ]]; then mark candidate_start
    elif [[ "$*" == *'systemctl stop llm-broker-blue'* ]]; then mark candidate_stop
    elif [[ "$*" == *'systemctl start llm-broker-green'* ]]; then mark previous_start
    else printf 'unexpected SSH invocation\n' >&2; return 2
    fi
}
'''


class BlueGreenDeployTests(unittest.TestCase):
    def run_case(self, failure=""):
        # Generated test resources stay inside this repository; only this exact
        # verified temporary directory is removed by TemporaryDirectory.
        with tempfile.TemporaryDirectory(prefix=".deploy-test-", dir=REPO) as raw:
            case = Path(raw).resolve()
            assert case.is_relative_to(REPO) and case.is_dir() and not case.is_symlink()
            (case / "bluegreen_deploy.sh").write_text(SCRIPT.read_text())
            (case / "common.sh").write_text(MOCK_COMMON)
            trace = case / "trace"
            result = subprocess.run(
                ["bash", str(case / "bluegreen_deploy.sh")],
                env={**os.environ, "CASE_DIR": str(case), "TRACE": str(trace), "FAIL_STAGE": failure},
                capture_output=True, text=True, timeout=10,
            )
            return result, trace.read_text().splitlines()

    def test_single_writer_handoff(self):
        result, trace = self.run_case()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertLess(trace.index("candidate_health"), trace.index("drain"))
        self.assertLess(trace.index("drain"), trace.index("switch"))
        self.assertLess(trace.index("switch"), trace.index("reload"))
        self.assertEqual(trace[-1], "smoke")
        self.assertIn("restore.sh snapshot-test", result.stdout)

    def test_unhealthy_candidate_preserves_previous_slot(self):
        result, trace = self.run_case("candidate_health")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("candidate_stop", trace)
        self.assertNotIn("drain", trace)
        self.assertNotIn("switch", trace)

    def test_failed_drain_restores_previous_service(self):
        result, trace = self.run_case("drain")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(trace[-2:], ["candidate_stop", "previous_start"])
        self.assertNotIn("switch", trace)

    def test_configuration_failure_restores_snapshot(self):
        for stage in ("switch", "reload", "public_health"):
            with self.subTest(stage=stage):
                result, trace = self.run_case(stage)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(trace[-1], "restore")
                self.assertNotIn("smoke", trace)


if __name__ == "__main__":
    unittest.main()
