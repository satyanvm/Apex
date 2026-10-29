"""Cancelling a run against a real Postgres test database
(AGENTLY_TEST_DATABASE_URL, migrated like docker-compose's).

Covers: a cancellation made in Postgres (as the API does) is relayed to the
workflow exactly once; a run with no workflow left is stamped too; the
reasoner's late writes never overwrite a canceled run; the cleanup marks only
unfinished nodes as stopped.
"""
from __future__ import annotations

import asyncio
import os
import unittest
from dataclasses import replace
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgresql://fake/fake")
os.environ.setdefault("GEMINI_API_KEY", "test-key")
os.environ.setdefault("LANGFUSE_PUBLIC_KEY", "test-public")
os.environ.setdefault("LANGFUSE_SECRET_KEY", "test-secret")
os.environ.setdefault("BROWSERBASE_API_KEY", "test-browserbase")
os.environ.setdefault("BROWSERBASE_PROJECT_ID", "test-project")
os.environ.setdefault("SMTP_HOST", "localhost")

from temporalio.service import RPCError, RPCStatusCode  # noqa: E402

from reasoner import db, dispatcher  # noqa: E402
from reasoner.config import CONFIG  # noqa: E402

TEST_DB = os.environ.get("AGENTLY_TEST_DATABASE_URL", "")


class _FakeHandle:
    def __init__(self, client, wf_id):
        self.client, self.wf_id = client, wf_id

    async def cancel(self):
        if self.wf_id in self.client.gone:
            raise RPCError("workflow not found for ID: " + self.wf_id, RPCStatusCode.NOT_FOUND, b"")
        self.client.cancels.append(self.wf_id)


class _FakeClient:
    def __init__(self):
        self.cancels: list[str] = []
        self.gone: set[str] = set()

    def get_workflow_handle(self, wf_id):
        return _FakeHandle(self, wf_id)


@unittest.skipUnless(TEST_DB, "AGENTLY_TEST_DATABASE_URL not set")
class CancelDbTest(unittest.TestCase):
    def setUp(self):
        cfg = replace(CONFIG, database_url=TEST_DB)
        self.patches = [mock.patch.object(m, "CONFIG", cfg) for m in (db, dispatcher)]
        for p in self.patches:
            p.start()
        self.ws, self.wf = db.gen_id("ws"), db.gen_id("wf")
        self.run = db.gen_id("run")
        asyncio.run(self._seed())

    def tearDown(self):
        asyncio.run(self._sql("delete from workspaces where id = %s", (self.ws,)))
        for p in self.patches:
            p.stop()

    async def _sql(self, q: str, args: tuple = ()):
        async with db._conn() as conn:
            cur = await conn.execute(q, args)
            return await cur.fetchall() if cur.description else None

    async def _seed(self):
        await self._sql("insert into workspaces (id, name, slug) values (%s, 't', %s)", (self.ws, self.ws))
        await self._sql("insert into workflows (id, workspace_id, slug, name) values (%s, %s, 'w', 'w')", (self.wf, self.ws))
        await self._sql("insert into runs (id, workspace_id, workflow_id, number, triggered_by, region, status) "
                        "values (%s, %s, %s, 1, '{}', 'local', 'running')", (self.run, self.ws, self.wf))
        for agent, status in (("ra_idle", "idle"), ("ra_busy", "running"), ("ra_done", "succeeded")):
            await self._sql("insert into run_agents (id, run_id, name, role, model, status) "
                            "values (%s, %s, %s, 'writer', 'none', %s)", (f"{agent}_{self.run}", self.run, agent, status))

    def cancel(self):
        """What the API's RunService.Cancel writes."""
        asyncio.run(self._sql("update runs set status='canceled', finished_at=now(), current_step='Canceled' "
                              "where id = %s", (self.run,)))

    def run_row(self):
        return asyncio.run(self._sql("select status, current_step, cancel_signaled_at from runs where id = %s",
                                     (self.run,)))[0]

    def test_cancel_is_relayed_to_the_workflow_once(self):
        client = _FakeClient()
        self.assertEqual(asyncio.run(dispatcher.relay_cancellations(client)), 0, "a running run is left alone")
        self.cancel()
        self.assertEqual(asyncio.run(dispatcher.relay_cancellations(client)), 1)
        self.assertEqual(client.cancels, [f"agently-run-{self.run}"])
        self.assertIsNotNone(self.run_row()[2], "cancel_signaled_at is stamped")
        self.assertEqual(asyncio.run(dispatcher.relay_cancellations(client)), 0, "delivered once")

    def test_cancel_of_a_run_with_no_workflow_is_stamped(self):
        # Canceled while still queued (never dispatched), or the workflow already finished.
        client = _FakeClient()
        client.gone.add(f"agently-run-{self.run}")
        self.cancel()
        self.assertEqual(asyncio.run(dispatcher.relay_cancellations(client)), 1)
        self.assertEqual(client.cancels, [])
        self.assertIsNotNone(self.run_row()[2])

    def test_a_canceled_run_is_never_overwritten(self):
        self.cancel()
        # A node that was still executing when the cancel landed finishes, and
        # the reasoner's usual writes arrive late.
        asyncio.run(db.set_run_running(self.run, "Running"))
        asyncio.run(db.set_run_progress(self.run, 5, 9, "Ran fetch"))
        asyncio.run(db.finish_run(self.run, "succeeded", "Completed"))
        status, step, _ = self.run_row()
        self.assertEqual((status, step), ("canceled", "Canceled"))

    def test_cleanup_stops_only_unfinished_nodes(self):
        self.cancel()
        self.assertEqual(asyncio.run(db.stop_canceled_run(self.run)), 2)
        rows = dict(asyncio.run(self._sql("select name, status from run_agents where run_id = %s", (self.run,))))
        self.assertEqual(rows, {"ra_idle": "blocked", "ra_busy": "blocked", "ra_done": "succeeded"})


if __name__ == "__main__":
    unittest.main()
