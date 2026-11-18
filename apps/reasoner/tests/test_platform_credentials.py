"""Credentials from the API's one-click Telegram connect: the platform marker
is replaced with the server's bot token, and that bot stays on its chat."""
from __future__ import annotations

import os
import unittest
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgresql://fake/fake")
os.environ.setdefault("GEMINI_API_KEY", "test-key")
os.environ.setdefault("LANGFUSE_PUBLIC_KEY", "test-public")
os.environ.setdefault("LANGFUSE_SECRET_KEY", "test-secret")
os.environ.setdefault("BROWSERBASE_API_KEY", "test-browserbase")
os.environ.setdefault("BROWSERBASE_PROJECT_ID", "test-project")

from reasoner import db  # noqa: E402
from reasoner.nodes import _platform_chat_guard  # noqa: E402


class FillPlatformValuesTest(unittest.TestCase):
    def test_marker_becomes_server_token(self) -> None:
        with mock.patch.dict(os.environ, {"TELEGRAM_BOT_TOKEN": " 123:abc "}):
            out = db.fill_platform_values({"TELEGRAM_BOT_TOKEN": "$platform:telegram", "chat_id": "42"})
        self.assertEqual(out, {"TELEGRAM_BOT_TOKEN": "123:abc", "chat_id": "42", "platform_bot": "telegram"})

    def test_unfillable_marker_is_empty(self) -> None:
        with mock.patch.dict(os.environ, {"TELEGRAM_BOT_TOKEN": ""}):
            self.assertEqual(db.fill_platform_values({"v": "$platform:telegram"})["v"], "")
        self.assertEqual(db.fill_platform_values({"v": "$platform:nope"})["v"], "")

    def test_plain_values_untouched(self) -> None:
        self.assertEqual(db.fill_platform_values({"TELEGRAM_BOT_TOKEN": "own"}), {"TELEGRAM_BOT_TOKEN": "own"})


class PlatformChatGuardTest(unittest.TestCase):
    cred = {"platform_bot": "telegram", "chat_id": "42"}

    def test_fills_empty_chat(self) -> None:
        cfg = {"chatId": "", "text": "hi"}
        _platform_chat_guard(self.cred, cfg)
        self.assertEqual(cfg["chatId"], "42")

    def test_refuses_other_chat(self) -> None:
        with self.assertRaisesRegex(RuntimeError, "only message the chat"):
            _platform_chat_guard(self.cred, {"chatId": "99"})

    def test_own_bot_unrestricted(self) -> None:
        cfg = {"chatId": "99"}
        _platform_chat_guard({"chat_id": "42"}, cfg)
        self.assertEqual(cfg["chatId"], "99")


if __name__ == "__main__":
    unittest.main()
