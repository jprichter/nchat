"""Offline regression tests for compose defaults and disclosed payload fields."""
import importlib.machinery
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader("compose", str(Path(__file__).with_name("compose")))
spec = importlib.util.spec_from_loader(loader.name, loader)
compose = importlib.util.module_from_spec(spec)
loader.exec_module(compose)


class ComposePrivacyTest(unittest.TestCase):
    def test_ollama_default_is_local_model(self):
        with patch("sys.argv", ["compose", "-s", "ollama", "-c", "chat.txt"]), \
                patch.object(compose, "read_chat_file", return_value=["Alice: hello", "Bob:"]), \
                patch.object(compose, "send_request", return_value="Hi") as send, \
                patch("builtins.print"):
            compose.main()
        payload, endpoint, _ = send.call_args.args
        self.assertEqual(endpoint, "http://localhost:11434/v1/chat/completions")
        self.assertEqual(payload["model"], "llama3.2:3b")
        self.assertIn({"role": "user", "content": "Alice: hello"}, payload["messages"])
        self.assertIn("Bob", payload["messages"][0]["content"])


if __name__ == "__main__":
    unittest.main()
