/**
 * What the worker does to a stored credential before a piece sees it, for
 * credentials made by the API's one-click connect flows
 * (apps/api/internal/services/oauth_connect.go, telegram_connect.go):
 *
 * - "$platform:telegram" values become the server's TELEGRAM_BOT_TOKEN. The
 *   API stores the marker so the server's token is never copied into anyone's
 *   credential, and refuses it in values a user types.
 * - An OAuth2 access token that has expired, or is about to, is refreshed with
 *   the stored refresh token and written back, so a connection keeps working
 *   past the hour most providers give an access token.
 */
import { createCipheriv, randomBytes } from 'node:crypto';

export const PLATFORM_PREFIX = '$platform:';

const PLATFORM_ENV: Record<string, string> = { telegram: 'TELEGRAM_BOT_TOKEN' };

/**
 * Replace platform markers with the server's values. A marker this server
 * can't fill becomes '' so the run fails as a missing credential instead of
 * sending the marker to a provider. Adds `platform_bot` naming the platform.
 */
export function fillPlatformValues(
  data: Record<string, unknown>,
  env: NodeJS.ProcessEnv = process.env,
): Record<string, unknown> {
  let platform: string | undefined;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(data)) {
    if (typeof v === 'string' && v.startsWith(PLATFORM_PREFIX)) {
      platform = v.slice(PLATFORM_PREFIX.length);
      const envKey = PLATFORM_ENV[platform];
      out[k] = envKey ? (env[envKey] ?? '').trim() : '';
    } else {
      out[k] = v;
    }
  }
  if (platform) out.platform_bot = platform;
  return out;
}

/**
 * The chat a platform-bot credential may message. Someone using the server's
 * bot can only reach the chat they connected, not every chat that ever
 * started the bot. Returns the props to run with, or an error message.
 */
export function platformChatProps(
  data: Record<string, unknown> | null | undefined,
  props: Record<string, unknown>,
  actionProps: Record<string, unknown> | undefined,
): { props: Record<string, unknown> } | { error: string } {
  if (!data || data.platform_bot !== 'telegram' || !actionProps || !('chat_id' in actionProps)) {
    return { props };
  }
  const connected = String(data.chat_id ?? '').trim();
  if (!connected) {
    return { error: 'This Telegram connection has no chat. Connect Telegram again from the Run dialog.' };
  }
  const given = String(props.chat_id ?? '').trim();
  if (!given) return { props: { ...props, chat_id: connected } };
  if (given !== connected) {
    return {
      error:
        `The Apex bot can only message the chat you connected (${connected}), not ${given}. ` +
        'Use that chat id, or connect your own bot to message other chats.',
    };
  }
  return { props };
}

/** Whether an OAuth2 credential's access token needs refreshing now. */
export function needsRefresh(data: Record<string, unknown>, nowSec: number = Date.now() / 1000): boolean {
  if (!data.refresh_token || !data.token_url || !data.client_id) return false;
  const claimed = Number(data.claimed_at);
  const expiresIn = Number(data.expires_in);
  if (!Number.isFinite(claimed) || !Number.isFinite(expiresIn) || expiresIn <= 0) return false;
  return nowSec >= claimed + expiresIn - 120;
}

/** Exchange the refresh token for a new access token; returns the updated data. */
export async function refreshOAuth(
  data: Record<string, unknown>,
  fetchFn: typeof fetch = fetch,
  nowSec: number = Math.floor(Date.now() / 1000),
): Promise<Record<string, unknown>> {
  const form = new URLSearchParams({
    grant_type: 'refresh_token',
    refresh_token: String(data.refresh_token),
  });
  const headers: Record<string, string> = {
    'content-type': 'application/x-www-form-urlencoded',
    accept: 'application/json',
  };
  const id = String(data.client_id ?? '');
  const secret = String(data.client_secret ?? '');
  if (String(data.authorization_method ?? '').toUpperCase() === 'HEADER') {
    headers.authorization = 'Basic ' + Buffer.from(`${id}:${secret}`).toString('base64');
  } else {
    form.set('client_id', id);
    form.set('client_secret', secret);
  }
  const res = await fetchFn(String(data.token_url), { method: 'POST', headers, body: form.toString() });
  const text = await res.text();
  let tok: Record<string, unknown> = {};
  try {
    tok = JSON.parse(text);
  } catch {
    tok = Object.fromEntries(new URLSearchParams(text));
  }
  if (!res.ok || !tok.access_token) {
    const why = String(tok.error_description ?? tok.error ?? `status ${res.status}`);
    throw new Error(`refreshing the access token failed: ${why}`);
  }
  const prevData = (data.data && typeof data.data === 'object' ? data.data : {}) as Record<string, unknown>;
  return {
    ...data,
    access_token: tok.access_token,
    // Most providers keep the refresh token; some rotate it.
    refresh_token: tok.refresh_token ?? data.refresh_token,
    expires_in: tok.expires_in ?? data.expires_in,
    token_type: tok.token_type ?? data.token_type,
    scope: tok.scope ?? data.scope,
    claimed_at: nowSec,
    data: { ...prevData, ...tok, refresh_token: tok.refresh_token ?? data.refresh_token },
  };
}

/**
 * Seal data the way the API does (platform/secretbox.go): AES-256-GCM,
 * `{"$enc": "v1:<base64(nonce || ciphertext || tag)>"}`. Plaintext JSON when
 * the server has no AGENTLY_CREDENTIALS_KEY, as the API stores it then.
 */
export function sealCredentialData(
  data: Record<string, unknown>,
  rawKey: string | undefined = process.env.AGENTLY_CREDENTIALS_KEY,
): unknown {
  const value = rawKey?.trim();
  if (!value) return data;
  let key: Buffer;
  if (/^[0-9a-fA-F]{64}$/.test(value)) key = Buffer.from(value, 'hex');
  else {
    key = Buffer.from(value, 'base64');
    if (key.length !== 32) throw new Error('AGENTLY_CREDENTIALS_KEY must be 32 bytes, hex or base64 encoded');
  }
  const nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  const ct = Buffer.concat([cipher.update(JSON.stringify(data), 'utf8'), cipher.final()]);
  return { $enc: 'v1:' + Buffer.concat([nonce, ct, cipher.getAuthTag()]).toString('base64') };
}
