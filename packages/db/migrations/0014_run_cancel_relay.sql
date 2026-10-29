-- 0014_run_cancel_relay.sql
--
-- Cancelling a run stops its Temporal workflow. The API only marks the run
-- canceled; the reasoner's dispatcher sends Temporal's cancel request to
-- agently-run-<id> and stamps this column, so each cancellation is relayed
-- once. Idempotent.
begin;

alter table runs add column if not exists cancel_signaled_at timestamptz;

commit;
