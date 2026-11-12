# Apex

Apex compiles a sentence into a workflow that runs.

> "When someone submits our Typeform, have AI score the lead. If it's above 70,
> create a HubSpot deal and alert sales on Slack."

That becomes a typed execution graph over 6,758 real integration actions. Every
node type and data reference is checked before anything is saved, and the run
survives a worker crash by resuming at the exact node.

## The problem it solves

Asking a model to write a workflow is easy. Getting one that runs is hard, for
three reasons Apex handles directly:

**The catalog doesn't fit in a prompt.** Apex narrows 6,758 actions and triggers
from 707 integrations in stages:

```text
6,758 actions   embedding search      100+ integrations
                small-model router    up to 16 integrations
                one map call each     up to 48 action schemas
                strong-model author   1 graph
```

**Models make things up.** Node types come from a closed catalog, and references
like `{{outputs.fetch.items}}` are checked against what the upstream node declares
it outputs. Validator errors go back to the model verbatim for up to two repair
rounds. Graphs too large for one model call are written section by section.

**Long-running work fails partway.** Each node is its own Temporal activity, and
independent branches run in parallel. Kill a worker and completed nodes are read
from workflow history rather than run again.

If a stage can't produce something valid, the compile fails and says why. There's
no fallback graph and no mock output.

## Measured

Three compiles through the public endpoint on November 8, 2026, on the production
Docker stack, using `gemini-3.1-flash-lite` for routing and mapping and
`gemini-3.5-flash` for writing the graph:

| Request | Nodes | Repair rounds | Compile time |
|---|---|---|---|
| Stripe charges to AI summary, Slack and Sheets | 7 | 0 | 28.7 s |
| GitHub bug to Sentry search, AI triage and Linear | 6 | 0 | 24.5 s |
| Typeform lead scoring with branches to HubSpot and Mailchimp | 7 | 2 | 51.7 s |

That's a sample of three, not a benchmark. In the third run the validator caught a
reference to an output field that didn't exist, and the repair loop fixed it. Try
your own in the playground; timing varies with the model provider's load.

The catalog counts come from `packages/nodes/pieces/index.json`: 5,304 actions and
1,454 triggers across 707 packages. The stage limits are the planner's defaults.

The test suites pass: 90 Go tests, 95 reasoner tests including Temporal ones, and
19 pieces-worker tests.

## Architecture

```text
Next.js web -> Go API -> Postgres
                           |
                    queued runs
                           v
               Python reasoner dispatcher
                           |
                 Temporal workflow history
                           |
               one activity per DAG node
                  /          |          \
              Gemini     Browserbase   pieces worker
```

- `apps/api`: Go planner, CRUD, schedules, triggers, logs and credentials.
- `apps/reasoner`: the run executor. It walks DAGs in deterministic Temporal
  workflow code and persists each node result as an activity checkpoint.
- `apps/pieces-worker`: runs Activepieces actions and triggers on its own queue.
- `apps/web`: the public site, the playground and the private console.
- `packages/nodes`: the integration catalogs and their embeddings.
- `packages/db/migrations`: the Postgres schema.

## Deploying

[DEPLOY.md](DEPLOY.md) takes a fresh server to HTTPS in seven steps. The public
playground only compiles: it's rate limited per visitor and per day, never saves
anything and never starts a run. Saving and running workflows needs a free
account (email and password), with daily limits per account.

## Failure model

- Planner failures return the provider's error to the operator. The public
  playground shows a safe summary and keeps the detail in the server log.
- Missing model credentials or stale embeddings stop the API at startup.
- Browser and email nodes without Browserbase or SMTP credentials fail that node
  with the reason. Langfuse tracing is optional.
- An unavailable pieces worker fails piece nodes.
- The app doesn't synthesize costs, logs, browser sessions or activity.

## Models

The planner uses `PLANNER_MAP_MODEL` (default `gemini-3.1-flash-lite`) for routing
and map selection, and `PLANNER_MODEL` for writing and repairing graphs. The
provider follows the model name, and any tier can run locally as `ollama/<model>`.
`PLANNER_EMBED_MODEL` provides catalog search; the committed embeddings use
`gemini-embedding-001`. The reasoner uses `REASONER_MODEL` and
`REASONER_SYNTHESIS_MODEL`. See `.env.example` for the tuning limits.

## Local development

```bash
cp .env.example .env
# Fill every required credential.
pnpm install
pnpm agently
```

Open the web app at `http://localhost:3000` and Temporal UI at
`http://localhost:8080`. See `RUNBOOK.md` for setup, migrations, service restart,
and verification commands, and its "Deploying" section for running the stack in
containers behind an access token and sign-in.
