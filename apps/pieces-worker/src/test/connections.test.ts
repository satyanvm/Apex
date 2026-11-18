import * as assert from 'node:assert/strict';
import { test } from 'node:test';
import { fillPlatformValues, needsRefresh, platformChatProps, refreshOAuth, sealCredentialData } from '../connections';
import { openCredentialData } from '../credentials';

const KEY = '000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f';

test('fillPlatformValues swaps the marker for the server token', () => {
  const out = fillPlatformValues(
    { value: '$platform:telegram', TELEGRAM_BOT_TOKEN: '$platform:telegram', chat_id: '42' },
    { TELEGRAM_BOT_TOKEN: ' 123:abc ' },
  );
  assert.deepEqual(out, { value: '123:abc', TELEGRAM_BOT_TOKEN: '123:abc', chat_id: '42', platform_bot: 'telegram' });
  assert.equal(fillPlatformValues({ value: '$platform:telegram' }, {}).value, '');
  assert.equal(fillPlatformValues({ value: '$platform:other' }, { TELEGRAM_BOT_TOKEN: 'x' }).value, '');
  assert.deepEqual(fillPlatformValues({ value: 'own-token' }, {}), { value: 'own-token' });
});

test('platformChatProps keeps the server bot on the connected chat', () => {
  const data = { platform_bot: 'telegram', chat_id: '42' };
  const actionProps = { chat_id: {}, message: {} };
  assert.deepEqual(platformChatProps(data, { message: 'hi' }, actionProps), { props: { message: 'hi', chat_id: '42' } });
  assert.deepEqual(platformChatProps(data, { chat_id: ' 42 ' }, actionProps), { props: { chat_id: ' 42 ' } });
  assert.match((platformChatProps(data, { chat_id: '99' }, actionProps) as { error: string }).error, /only message the chat/);
  // Own bots and actions without a chat id are untouched.
  assert.deepEqual(platformChatProps({ chat_id: '42' }, { chat_id: '99' }, actionProps), { props: { chat_id: '99' } });
  assert.deepEqual(platformChatProps(data, { x: 1 }, { x: {} }), { props: { x: 1 } });
});

test('needsRefresh only for expiring OAuth credentials that can refresh', () => {
  const base = { refresh_token: 'r', token_url: 'https://t', client_id: 'c', claimed_at: 1000, expires_in: 3600 };
  assert.equal(needsRefresh(base, 1000 + 3600 - 200), false);
  assert.equal(needsRefresh(base, 1000 + 3600 - 60), true);
  assert.equal(needsRefresh({ ...base, refresh_token: undefined }, 99999), false);
  assert.equal(needsRefresh({ value: 'secret' }, 99999), false);
});

test('refreshOAuth posts the refresh grant and keeps the old refresh token', async () => {
  let seen: { url: string; body: string; auth?: string } | undefined;
  const fakeFetch = (async (url: string, init: RequestInit) => {
    seen = { url, body: String(init.body), auth: (init.headers as Record<string, string>).authorization };
    return new Response(JSON.stringify({ access_token: 'new', expires_in: 3599 }), { status: 200 });
  }) as unknown as typeof fetch;
  const data = {
    access_token: 'old', refresh_token: 'r1', token_url: 'https://oauth.test/token', client_id: 'cid',
    client_secret: 'cs', claimed_at: 1, expires_in: 3600, data: { id_token: 'x' },
  };
  const out = await refreshOAuth(data, fakeFetch, 5000);
  const body = new URLSearchParams(seen!.body);
  assert.equal(seen!.url, 'https://oauth.test/token');
  assert.equal(body.get('grant_type'), 'refresh_token');
  assert.equal(body.get('refresh_token'), 'r1');
  assert.equal(body.get('client_secret'), 'cs');
  assert.equal(out.access_token, 'new');
  assert.equal(out.refresh_token, 'r1');
  assert.equal(out.claimed_at, 5000);
  assert.equal((out.data as Record<string, unknown>).id_token, 'x');

  await refreshOAuth({ ...data, authorization_method: 'HEADER' }, fakeFetch, 1);
  assert.equal(seen!.auth, 'Basic ' + Buffer.from('cid:cs').toString('base64'));
  assert.equal(new URLSearchParams(seen!.body).get('client_secret'), null);

  const failing = (async () => new Response('{"error":"invalid_grant"}', { status: 400 })) as unknown as typeof fetch;
  await assert.rejects(refreshOAuth(data, failing), /invalid_grant/);
});

test('sealCredentialData round-trips through openCredentialData', () => {
  const data = { access_token: 'a', nested: { x: 1 } };
  const sealed = sealCredentialData(data, KEY) as Record<string, unknown>;
  assert.match(String(sealed.$enc), /^v1:/);
  assert.deepEqual(openCredentialData(sealed, KEY), data);
  assert.deepEqual(sealCredentialData(data, ''), data);
});
