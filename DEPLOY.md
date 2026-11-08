# Deploying Apex

The whole stack runs on one server with Docker Compose. Caddy gets the HTTPS
certificate by itself, and it is the only service reachable from the internet.

## What you need

- A server with at least 2 vCPUs and 4 GB of RAM, running Ubuntu 24.04
- A domain you can edit DNS for
- A Gemini API key from https://aistudio.google.com/apikey

## Steps

**1. Point the domain at the server.** Create an `A` record for your domain with
the server's IP address. If you use Cloudflare, set it to "DNS only" (grey cloud).
Behind its proxy, every visitor would look like the same few IPs and share one
rate limit.

**2. Install Docker and open the ports.** Run on the server:

```bash
curl -fsSL https://get.docker.com | sh
```

```bash
ufw allow 22,80,443/tcp && ufw --force enable
```

**3. Get the code.**

```bash
git clone -b launch/public-site https://github.com/satyanvm/Apex.git && cd Apex
```

**4. Generate the config.** Replace the domain with yours. This creates `.env`
with random secrets.

```bash
./scripts/init-prod-env.sh yourdomain.com
```

**5. Add your Gemini key.** Open `.env` and fill in `GEMINI_API_KEY`.

```bash
nano .env
```

**6. Start it.** The first build takes several minutes.

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

**7. Check it.** Open `https://yourdomain.com/playground` and compile a workflow.

```bash
docker compose -f docker-compose.prod.yml ps
```

`migrate` should show as exited (0), and everything else as running. The first
visit can take a few seconds while Caddy gets the certificate.

## What's public

| Path | Access |
|---|---|
| `/`, `/playground`, `/api/public/*`, `/api/health` | Anyone |
| `/api/hooks/*` | Anyone, because third-party webhooks call it |
| Everything else, including the console at `/dashboard` | `AGENTLY_WEB_PASSWORD`, with any username |

The public playground only compiles. It never saves a workflow or starts a run.

## Controlling cost

These limits are in `.env`. Restart the API after changing them with
`docker compose -f docker-compose.prod.yml up -d api`.

| Setting | Default | Meaning |
|---|---|---|
| `PUBLIC_COMPILE_PER_HOUR` | 6 | Fresh compiles per visitor per hour |
| `PUBLIC_COMPILE_DAILY_BUDGET` | 200 | Fresh compiles per UTC day, for everyone combined |
| `PUBLIC_COMPILE_MAX_CONCURRENT` | 3 | Compiles running at once |
| `PUBLIC_COMPILE_MAX_PROMPT` | 1200 | Longest prompt, in characters |
| `PUBLIC_COMPILE_MAX_NODES` | 40 | Largest graph the demo will write |

Repeated prompts are served from memory and cost nothing. Counters are held in
memory, so restarting the API resets them.

Set a spending cap on your Gemini key too. The limits above bound how many
compiles run, not what your provider charges for each one.

## Optional services

The stack boots without these. A run that reaches a node needing one fails that
node and names what's missing.

| Service | Variables | Needed for |
|---|---|---|
| Browserbase | `BROWSERBASE_API_KEY`, `BROWSERBASE_PROJECT_ID` | Browser nodes |
| SMTP | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` | Email nodes |
| Langfuse | `LANGFUSE_HOST`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY` | Tracing LLM calls |

## Updating

```bash
git pull && docker compose -f docker-compose.prod.yml up -d --build
```

New database migrations are applied automatically on start.

## Backups

Keep a copy of `.env`. Without `AGENTLY_CREDENTIALS_KEY`, stored credentials
can't be decrypted. To back up the database:

```bash
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U agently agently > apex-backup.sql
```
