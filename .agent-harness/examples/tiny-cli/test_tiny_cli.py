import unittest

from tiny_cli import greeting


class TinyCliTest(unittest.TestCase):
    def test_greeting_uses_name(self):
        self.assertEqual(greeting("Codex"), "hello, Codex")

    def test_greeting_defaults_blank_name(self):
        self.assertEqual(greeting("   "), "hello, agent")


if __name__ == "__main__":
    unittest.main()

