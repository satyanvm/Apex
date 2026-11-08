#!/bin/sh
# Writes a production .env with freshly generated secrets.
#   ./scripts/init-prod-env.sh yourdomain.com
set -eu

DOMAIN="${1:-}"
if [ -z "$DOMAIN" ]; then
  echo "usage: $0 yourdomain.com" >&2
  exit 1
fi
if [ -e .env ]; then
  echo ".env already exists. Move it aside first; it holds your encryption key." >&2
  exit 1
fi

secret() { openssl rand -hex 32; }

umask 077
cat > .env <<ENV
DOMAIN=$DOMAIN

# Required: https://aistudio.google.com/apikey
GEMINI_API_KEY=

# Password for the private console (any username).
AGENTLY_WEB_PASSWORD=$(openssl rand -hex 16)

# Generated. Back up AGENTLY_CREDENTIALS_KEY: stored credentials can't be read without it.
AGENTLY_API_TOKEN=$(secret)
AGENTLY_CREDENTIALS_KEY=$(secret)
POSTGRES_PASSWORD=$(secret)
TEMPORAL_DB_PASSWORD=$(secret)

# Public playground limits.
PUBLIC_COMPILE_PER_HOUR=6
PUBLIC_COMPILE_DAILY_BUDGET=200
PUBLIC_COMPILE_MAX_CONCURRENT=3

# Optional. Without these, only the nodes that need them fail, with the reason.
BROWSERBASE_API_KEY=
BROWSERBASE_PROJECT_ID=
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASS=
SMTP_FROM=
LANGFUSE_HOST=
LANGFUSE_PUBLIC_KEY=
LANGFUSE_SECRET_KEY=
ENV

echo "Wrote .env for $DOMAIN. Add your GEMINI_API_KEY, then run:"
echo "  docker compose -f docker-compose.prod.yml up -d --build"
