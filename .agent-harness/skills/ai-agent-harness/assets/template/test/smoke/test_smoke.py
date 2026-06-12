import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def run_command(args):
    return subprocess.run(args, cwd=ROOT, text=True, capture_output=True)


class SmokeTests(unittest.TestCase):
    def test_primary_template_commands(self):
        commands = [
            ["python3", "scripts/validate-state.py"],
            ["scripts/summarize-progress.sh"],
            ["scripts/summarize-runs.sh"],
            ["scripts/check-failure-domains.sh"],
        ]
        for command in commands:
            with self.subTest(command=command):
                result = run_command(command)
                self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
