import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
INIT_SCRIPT = ROOT / "skills" / "ai-agent-harness" / "scripts" / "init_harness.py"


def run_initializer(project: Path, mode: str, *extra: str):
    return subprocess.run(
        [
            sys.executable,
            str(INIT_SCRIPT),
            "--root",
            str(project),
            "--template-root",
            str(ROOT),
            "--mode",
            mode,
            *extra,
        ],
        cwd=ROOT,
        text=True,
        capture_output=True,
    )


def run_initializer_with_template(project: Path, template: Path, mode: str, *extra: str):
    return subprocess.run(
        [
            sys.executable,
            str(INIT_SCRIPT),
            "--root",
            str(project),
            "--template-root",
            str(template),
            "--mode",
            mode,
            *extra,
        ],
        cwd=ROOT,
        text=True,
        capture_output=True,
    )


def run_project_init(project: Path):
    return subprocess.run(
        [str(project / "init.sh")],
        cwd=project,
        text=True,
        capture_output=True,
        env={**os.environ, "HARNESS_SKIP_TEST_LAYERS": "1"},
    )


class SkillInitializerHarnessTests(unittest.TestCase):
    EXECUTABLE_PATHS = [
        "init.sh",
        ".agent-harness/scripts/check-evaluator-evidence.sh",
        ".agent-harness/init.sh",
        ".agent-harness/scripts/check-failure-domains.sh",
        ".agent-harness/scripts/init.sh",
        ".agent-harness/scripts/run-coding-agent.sh",
        ".agent-harness/scripts/run-evaluator-agent.sh",
        ".agent-harness/scripts/summarize-progress.sh",
        ".agent-harness/scripts/summarize-runs.sh",
        ".agent-harness/scripts/validate-feature.sh",
    ]

    def test_new_mode_creates_runnable_harness_and_clean_check(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            project = Path(tmp_dir)
            result = run_initializer(project, "new")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("mode=new", result.stdout)
            self.assertIn("layout=hidden", result.stdout)
            self.assertIn("state_valid=true", result.stdout)
            self.assertIn("runnable_harness=true", result.stdout)
            self.assertIn("state_reset=.agent-harness/feature_list.json,.agent-harness/progress.md", result.stdout)

            data = json.loads((project / ".agent-harness" / "feature_list.json").read_text())
            self.assertEqual(data, {"features": []})
            manifest = json.loads((project / ".agent-harness" / "manifest.json").read_text())
            self.assertEqual(manifest["template_version"], "0.3.3")
            self.assertEqual(manifest["layout"], "hidden")
            self.assertIn("category", manifest["files"][".agent-harness/scripts/validate-state.py"])
            self.assertTrue((project / "AGENTS.md").exists())
            self.assertTrue((project / "init.sh").exists())
            self.assertTrue((project / ".agent-harness" / "docs" / "project-recovery-init.md").exists())
            self.assertTrue((project / ".agent-harness" / "docs" / "spec-normalization.md").exists())
            self.assertIn("project recovery contract", (project / "AGENTS.md").read_text())
            self.assertIn("Spec Normalization", (project / "AGENTS.md").read_text())
            self.assertIn("Pre-minspec state", (project / "init.sh").read_text())
            self.assertIn(
                "root `./init.sh` may only verify the harness",
                (project / ".agent-harness" / "docs" / "project-recovery-init.md").read_text(),
            )
            self.assertIn(
                "Required Fields",
                (project / ".agent-harness" / "docs" / "spec-normalization.md").read_text(),
            )
            self.assertIn(
                "Reject vague requirements",
                (project / ".agent-harness" / "prompts" / "plan.md").read_text(),
            )
            for rel in self.EXECUTABLE_PATHS:
                self.assertTrue(os.access(project / rel, os.X_OK), f"{rel} should be executable")
            self.assertFalse((project / "feature_list.json").exists())
            self.assertFalse((project / "scripts").exists())

            init = run_project_init(project)
            self.assertEqual(init.returncode, 0, init.stderr)

            check = run_initializer(project, "check")
            self.assertEqual(check.returncode, 0, check.stdout + check.stderr)
            self.assertIn("layout=hidden", check.stdout)
            self.assertIn("template_version=0.3.3", check.stdout)
            self.assertIn("installed_version=0.3.3", check.stdout)
            self.assertIn("state_valid=true", check.stdout)
            self.assertIn("runnable_harness=true", check.stdout)
            self.assertIn("project_state_changed=", check.stdout)
            self.assertIn("next_action=harness is installed and runnable", check.stdout)

    def test_hidden_layout_repairs_executable_bits_when_template_modes_are_lost(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp = Path(tmp_dir)
            template = tmp / "template"
            project = tmp / "project"
            shutil.copytree(
                ROOT,
                template,
                ignore=shutil.ignore_patterns(".git", "__pycache__", ".pytest_cache"),
            )
            for rel in [
                "init.sh",
                "scripts/check-evaluator-evidence.sh",
                "scripts/check-failure-domains.sh",
                "scripts/init.sh",
                "scripts/run-coding-agent.sh",
                "scripts/run-evaluator-agent.sh",
                "scripts/summarize-progress.sh",
                "scripts/summarize-runs.sh",
                "scripts/validate-feature.sh",
            ]:
                (template / rel).chmod(0o644)

            result = run_initializer_with_template(project, template, "new")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn("state_valid=true", result.stdout)
            self.assertIn("runnable_harness=true", result.stdout)
            for rel in self.EXECUTABLE_PATHS:
                self.assertTrue(os.access(project / rel, os.X_OK), f"{rel} should be executable")
            init = run_project_init(project)
            self.assertEqual(init.returncode, 0, init.stderr)

    def test_visible_layout_remains_available_for_harness_development(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            project = Path(tmp_dir)
            result = run_initializer(project, "new", "--layout", "visible")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("layout=visible", result.stdout)
            self.assertIn("state_reset=feature_list.json,progress.md", result.stdout)
            self.assertTrue((project / "feature_list.json").exists())
            self.assertTrue((project / "scripts" / "init.sh").exists())
            self.assertTrue((project / "prompts" / "work.md").exists())
            manifest = json.loads((project / ".agent-harness" / "manifest.json").read_text())
            self.assertEqual(manifest["layout"], "visible")

    def test_adopt_mode_does_not_overwrite_merge_sensitive_files_by_default(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            project = Path(tmp_dir)
            (project / "README.md").write_text("existing readme\n")
            (project / "AGENTS.md").write_text("existing agent rules\n")

            result = run_initializer(project, "adopt")
            self.assertEqual(result.returncode, 1)
            self.assertIn("blocking_conflicts:", result.stdout)
            self.assertIn("AGENTS.md", result.stdout)
            self.assertEqual((project / "README.md").read_text(), "existing readme\n")
            self.assertEqual((project / "AGENTS.md").read_text(), "existing agent rules\n")
            self.assertFalse((project / ".agent-harness" / "manifest.json").exists())
            self.assertFalse((project / ".agent-harness" / "feature_list.json").exists())

            forced = run_initializer(project, "adopt", "--force")
            self.assertEqual(forced.returncode, 0, forced.stderr)
            self.assertIn("overwritten=1", forced.stdout)
            self.assertEqual((project / "README.md").read_text(), "existing readme\n")
            self.assertNotEqual((project / "AGENTS.md").read_text(), "existing agent rules\n")

    def test_repair_restores_missing_harness_files_and_preserves_project_state(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            project = Path(tmp_dir)
            initial = run_initializer(project, "new")
            self.assertEqual(initial.returncode, 0, initial.stderr)

            custom_features = {
                "features": [
                    {
                        "id": "F999",
                        "title": "Custom project feature",
                        "description": "Project-owned feature state must survive repair.",
                        "acceptance": ["state remains intact"],
                        "passes": False,
                        "status": "todo",
                        "attempts": 7,
                        "last_error": "keep me",
                    }
                ]
            }
            (project / ".agent-harness" / "feature_list.json").write_text(json.dumps(custom_features, indent=2) + "\n")
            (project / ".agent-harness" / "progress.md").write_text(
                "# Progress\n\n"
                "## Current System Status\n\ncustom\n\n"
                "## Last Completed Feature\n\nNone.\n\n"
                "## Next Feature\n\nF999\n\n"
                "## Known Issues\n\nNone.\n"
            )
            (project / ".agent-harness" / "scripts" / "validate-state.py").unlink()
            (project / ".agent-harness" / "prompts" / "work.md").unlink()

            repaired = run_initializer(project, "repair")
            self.assertEqual(repaired.returncode, 0, repaired.stdout + repaired.stderr)
            self.assertIn("mode=repair", repaired.stdout)
            self.assertIn("created=2", repaired.stdout)
            self.assertTrue((project / ".agent-harness" / "scripts" / "validate-state.py").exists())
            self.assertTrue((project / ".agent-harness" / "prompts" / "work.md").exists())
            self.assertEqual(json.loads((project / ".agent-harness" / "feature_list.json").read_text()), custom_features)
            self.assertIn("F999", (project / ".agent-harness" / "progress.md").read_text())

    def test_check_reports_complete_diagnostics_and_version_drift(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            project = Path(tmp_dir)
            initial = run_initializer(project, "new")
            self.assertEqual(initial.returncode, 0, initial.stderr)

            manifest_path = project / ".agent-harness" / "manifest.json"
            manifest = json.loads(manifest_path.read_text())
            manifest["template_version"] = "0.0.0"
            manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
            (project / ".agent-harness" / "scripts" / "validate-state.py").write_text("local static drift\n")
            (project / "AGENTS.md").write_text("local merge-sensitive change\n")

            check = run_initializer(project, "check")
            self.assertEqual(check.returncode, 1)
            for phrase in [
                "mode=check",
                "template_version=0.3.3",
                "installed_version=0.0.0",
                "state_valid=false",
                "runnable_harness=false",
                "missing=0",
                "conflicts=1",
                "drift=1",
                "project_state_changed=",
                "optional_changed=0",
                "state_errors:",
                "next_action=review merge-sensitive conflicts",
            ]:
                self.assertIn(phrase, check.stdout)


if __name__ == "__main__":
    unittest.main()
