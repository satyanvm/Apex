-- 0016_accounts.sql — email + password sign-in.
--
-- An account is a member with a password. Signing up creates both rows; the
-- member row is what workflows (owner_id) and runs already point at.
-- Sessions store only a SHA-256 of the cookie token, so a database read does
-- not hand out working sessions.
--
-- credentials.owner_id makes saved accounts private to the member who saved
-- them. Rows from before this migration have no owner and only workflows with
-- no account owner (the operator's own) can still use them.
begin;

create table if not exists accounts (
  member_id      text primary key references members(id) on delete cascade,
  email          text not null,
  password_hash  text not null,
  created_at     timestamptz not null default now()
);

create unique index if not exists accounts_email_idx on accounts (lower(email));

create table if not exists auth_sessions (
  token_hash  text primary key,
  member_id   text not null references accounts(member_id) on delete cascade,
  expires_at  timestamptz not null,
  created_at  timestamptz not null default now()
);

create index if not exists auth_sessions_member_idx on auth_sessions (member_id);
create index if not exists auth_sessions_expiry_idx on auth_sessions (expires_at);

alter table credentials add column if not exists owner_id text references members(id) on delete cascade;
create index if not exists credentials_owner_idx on credentials (owner_id);

commit;
