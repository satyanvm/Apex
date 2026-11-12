package services

// account_service.go — email + password accounts and cookie sessions.
//
// Passwords are hashed with PBKDF2-SHA256 (standard library, OWASP's 600k
// iterations). Session tokens are 32 random bytes; only their SHA-256 is stored.

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/platform"
)

const (
	SessionTTL        = 30 * 24 * time.Hour
	passwordMinLen    = 8
	passwordMaxLen    = 128
	pbkdf2Iterations  = 600_000
	pbkdf2KeyLen      = 32
	passwordHashLabel = "pbkdf2-sha256"
)

type AccountService struct {
	deps Deps
	now  func() time.Time
}

// dummyHash is checked against when an email has no account, so a sign-in
// takes as long whether or not the email is registered.
var dummyHash = sync.OnceValue(func() string {
	h, _ := hashPassword("not-a-real-password")
	return h
})

func NewAccountService(deps Deps) *AccountService {
	return &AccountService{deps: deps, now: time.Now}
}

// SignUp creates an account and signs it in, returning the session token.
func (s *AccountService) SignUp(name, email, password string) (domain.Member, string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return domain.Member{}, "", err
	}
	if n := utf8.RuneCountInString(password); n < passwordMinLen || n > passwordMaxLen {
		return domain.Member{}, "", domain.BadRequest(fmt.Sprintf("Password must be %d to %d characters.", passwordMinLen, passwordMaxLen))
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if utf8.RuneCountInString(name) > 80 {
		return domain.Member{}, "", domain.BadRequest("Name must be at most 80 characters.")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.Member{}, "", err
	}
	m := domain.Member{
		ID: domain.NewMemberId(), WorkspaceID: s.deps.Repos.Workspace.Get().ID,
		Name: name, Email: email, Initials: initialsOf(name), Role: domain.MemberMember,
		CreatedAt: domain.Timestamp(s.deps.Clock.ISO()),
	}
	if err := s.deps.Repos.Accounts.Create(m, hash); err != nil {
		if errors.Is(err, platform.ErrEmailTaken) {
			return domain.Member{}, "", domain.Conflict("An account with this email already exists. Sign in instead.")
		}
		return domain.Member{}, "", domain.Errorf(domain.CodeInternal, "could not create the account")
	}
	token, err := s.startSession(m.ID)
	return m, token, err
}

// SignIn checks the password and returns a new session token.
func (s *AccountService) SignIn(email, password string) (domain.Member, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	m, hash, ok := s.deps.Repos.Accounts.ByEmail(email)
	if !ok {
		hash = dummyHash()
	}
	if !checkPassword(hash, password) || !ok {
		return domain.Member{}, "", domain.Unauthorized("Wrong email or password.")
	}
	token, err := s.startSession(m.ID)
	return m, token, err
}

func (s *AccountService) SignOut(token string) {
	if token != "" {
		s.deps.Repos.Accounts.DeleteSession(hashToken(token))
	}
}

// MemberForToken returns the signed-in member for a session token.
func (s *AccountService) MemberForToken(token string) (domain.Member, bool) {
	if token == "" {
		return domain.Member{}, false
	}
	return s.deps.Repos.Accounts.SessionMember(hashToken(token), s.now())
}

// HasAccount reports whether a member can sign in, as opposed to the members
// the seed data and the pre-accounts console created.
func (s *AccountService) HasAccount(id domain.MemberId) bool {
	return s.deps.Repos.Accounts.Exists(id)
}

func (s *AccountService) startSession(member domain.MemberId) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.deps.Repos.Accounts.CreateSession(hashToken(token), member, s.now().Add(SessionTTL)); err != nil {
		return "", domain.Errorf(domain.CodeInternal, "could not start a session")
	}
	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", domain.BadRequest("Enter a valid email address.")
	}
	return email, nil
}

func initialsOf(name string) string {
	out := []rune{}
	for _, word := range strings.Fields(name) {
		r, _ := utf8.DecodeRuneInString(word)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, unicode.ToUpper(r))
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

// hashPassword returns "pbkdf2-sha256$<iterations>$<salt>$<key>" (base64).
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("%s$%d$%s$%s", passwordHashLabel, pbkdf2Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func checkPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != passwordHashLabel {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
