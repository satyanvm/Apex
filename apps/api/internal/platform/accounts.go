package platform

// accounts.go — the in-memory and Postgres AccountRepo (migration 0016).

import (
	"errors"
	"strings"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

/* ------------------------------ in memory ------------------------------ */

type memSession struct {
	member    domain.MemberId
	expiresAt time.Time
}

type accountRepo struct{ s *store }

func (r *accountRepo) Create(m domain.Member, passwordHash string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, existing := range r.s.members {
		if strings.EqualFold(existing.Email, m.Email) {
			return ErrEmailTaken
		}
	}
	if r.s.accounts == nil {
		r.s.accounts = map[domain.MemberId]string{}
	}
	r.s.members = append(r.s.members, m)
	r.s.accounts[m.ID] = passwordHash
	return nil
}

func (r *accountRepo) ByEmail(email string) (domain.Member, string, bool) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	for _, m := range r.s.members {
		if hash, ok := r.s.accounts[m.ID]; ok && strings.EqualFold(m.Email, email) {
			return m, hash, true
		}
	}
	return domain.Member{}, "", false
}

func (r *accountRepo) Exists(id domain.MemberId) bool {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	_, ok := r.s.accounts[id]
	return ok
}

func (r *accountRepo) CreateSession(tokenHash string, member domain.MemberId, expiresAt time.Time) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.authSessions == nil {
		r.s.authSessions = map[string]memSession{}
	}
	r.s.authSessions[tokenHash] = memSession{member: member, expiresAt: expiresAt}
	return nil
}

func (r *accountRepo) SessionMember(tokenHash string, now time.Time) (domain.Member, bool) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	sess, ok := r.s.authSessions[tokenHash]
	if !ok || !now.Before(sess.expiresAt) {
		return domain.Member{}, false
	}
	for _, m := range r.s.members {
		if m.ID == sess.member {
			return m, true
		}
	}
	return domain.Member{}, false
}

func (r *accountRepo) DeleteSession(tokenHash string) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	delete(r.s.authSessions, tokenHash)
}

/* ------------------------------- Postgres ------------------------------ */

type pgAccountRepo struct{ s *pgStore }

func (r *pgAccountRepo) Create(m domain.Member, passwordHash string) error {
	ctx := bg()
	tx, err := r.s.pool.Begin(ctx)
	if err != nil {
		r.s.fail("account.Create begin", err)
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var taken bool
	if err := tx.QueryRow(ctx,
		`select exists(select 1 from accounts where lower(email)=lower($1))
		     or exists(select 1 from members where workspace_id=$2 and lower(email)=lower($1))`,
		m.Email, string(m.WorkspaceID)).Scan(&taken); err != nil {
		r.s.fail("account.Create check", err)
		return err
	}
	if taken {
		return ErrEmailTaken
	}
	if _, err := tx.Exec(ctx,
		`insert into members (`+memberCols+`) values ($1,$2,$3,$4,$5,$6,$7)`,
		string(m.ID), string(m.WorkspaceID), m.Name, m.Email, m.Initials, string(m.Role), tsArg(m.CreatedAt)); err != nil {
		return uniqueAsTaken(r.s, "account.Create member", err)
	}
	if _, err := tx.Exec(ctx,
		`insert into accounts (member_id, email, password_hash) values ($1,$2,$3)`,
		string(m.ID), m.Email, passwordHash); err != nil {
		return uniqueAsTaken(r.s, "account.Create account", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uniqueAsTaken(r.s, "account.Create commit", err)
	}
	return nil
}

// uniqueAsTaken maps a unique violation (two sign-ups racing on one email) to
// ErrEmailTaken and logs anything else.
func uniqueAsTaken(s *pgStore, op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	s.fail(op, err)
	return err
}

func (r *pgAccountRepo) ByEmail(email string) (domain.Member, string, bool) {
	row := r.s.pool.QueryRow(bg(),
		`select m.id, m.workspace_id, m.name, m.email, m.initials, m.role, m.created_at, a.password_hash
		   from accounts a join members m on m.id = a.member_id
		  where lower(a.email) = lower($1)`, email)
	var m domain.Member
	var id, ws, name, mail, initials, role, hash string
	var created any
	if err := row.Scan(&id, &ws, &name, &mail, &initials, &role, &created, &hash); err != nil {
		return domain.Member{}, "", false
	}
	m.ID, m.WorkspaceID = domain.MemberId(id), domain.WorkspaceId(ws)
	m.Name, m.Email, m.Initials, m.Role = name, mail, initials, domain.MemberRole(role)
	m.CreatedAt = anyTs(created)
	return m, hash, true
}

func (r *pgAccountRepo) Exists(id domain.MemberId) bool {
	var ok bool
	err := r.s.pool.QueryRow(bg(), `select exists(select 1 from accounts where member_id=$1)`, string(id)).Scan(&ok)
	r.s.fail("account.Exists", err)
	return err == nil && ok
}

func (r *pgAccountRepo) CreateSession(tokenHash string, member domain.MemberId, expiresAt time.Time) error {
	// Expired sessions are cleared here, at sign-in, rather than by a job.
	_, _ = r.s.pool.Exec(bg(), `delete from auth_sessions where expires_at < now()`)
	_, err := r.s.pool.Exec(bg(),
		`insert into auth_sessions (token_hash, member_id, expires_at) values ($1,$2,$3)`,
		tokenHash, string(member), expiresAt)
	r.s.fail("account.CreateSession", err)
	return err
}

func (r *pgAccountRepo) SessionMember(tokenHash string, now time.Time) (domain.Member, bool) {
	row := r.s.pool.QueryRow(bg(),
		`select `+prefixed("m.", memberCols)+`
		   from auth_sessions s join members m on m.id = s.member_id
		  where s.token_hash = $1 and s.expires_at > $2`, tokenHash, now)
	m, err := scanMember(row)
	if err != nil {
		return domain.Member{}, false
	}
	return m, true
}

func (r *pgAccountRepo) DeleteSession(tokenHash string) {
	_, err := r.s.pool.Exec(bg(), `delete from auth_sessions where token_hash=$1`, tokenHash)
	r.s.fail("account.DeleteSession", err)
}

// prefixed qualifies each column of a comma-separated list with a table alias.
func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, c := range parts {
		parts[i] = alias + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}
