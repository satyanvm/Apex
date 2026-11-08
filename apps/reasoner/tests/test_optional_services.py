"""Optional services: absent credentials must not stop the reasoner booting."""
import importlib
import os
import unittest
from dataclasses import replace
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgresql://u:p@localhost/test")
os.environ.setdefault("GEMINI_API_KEY", "test-key")

from reasoner import browser, config, obs  # noqa: E402


class OptionalServicesTest(unittest.TestCase):
    def test_config_loads_without_optional_services(self):
        absent = ("LANGFUSE_PUBLIC_KEY", "LANGFUSE_SECRET_KEY", "BROWSERBASE_API_KEY",
                  "BROWSERBASE_PROJECT_ID", "SMTP_HOST")
        env = {k: v for k, v in os.environ.items() if k not in absent}
        with mock.patch.dict(os.environ, env, clear=True):
            cfg = config.load()
        self.assertFalse(cfg.langfuse_enabled)
        self.assertFalse(cfg.browserbase_enabled)
        self.assertFalse(cfg.smtp_enabled)

    def test_untraced_when_langfuse_is_not_configured(self):
        cfg = replace(obs.CONFIG, langfuse_public_key="", langfuse_secret_key="")
        with mock.patch.object(obs, "CONFIG", cfg):
            self.assertNotIn("callbacks", obs.langchain_config("run", "node"))
            self.assertEqual(obs.session_handle("run"), "")
            obs.flush()


class BrowserWithoutCredentials(unittest.IsolatedAsyncioTestCase):
    async def test_browser_node_fails_with_the_reason(self):
        # test_engine swaps run_browse for a fake and leaves it; get the real one.
        module = importlib.reload(browser)
        cfg = replace(module.CONFIG, browserbase_api_key="", browserbase_project_id="")
        with mock.patch.object(module, "CONFIG", cfg):
            with self.assertRaisesRegex(RuntimeError, "BROWSERBASE_API_KEY"):
                await module.run_browse("run", "label", ["https://example.com"])


if __name__ == "__main__":
    unittest.main()
