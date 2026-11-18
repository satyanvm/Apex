package services

// telegram_connect.go — connect Telegram without pasting tokens or chat ids.
//
// Telegram has no OAuth for bots, so the connect flow uses a deep link instead:
//
//  1. The web app asks for a one-time code (POST /api/connect/telegram/start).
//  2. The user opens t.me/<bot>?start=<code> and taps Start. Telegram sends the
//     bot "/start <code>" from the user's own chat.
//  3. The API, long-polling that bot's updates, matches the code, saves the
//     credential with the chat id it came from, and replies in the chat.
//  4. The web app polls GET /api/connect/telegram/status?code=… until saved.
//
// With TELEGRAM_BOT_TOKEN set, the server's own bot is used and the user types
// nothing: the credential stores the "$platform:telegram" marker, which the
// runtimes replace with the server's token, so the token is never copied into
// anyone's credential. Without it, or by choice, the user pastes their own
// BotFather token once and the same deep link finds the chat id.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/platform"
)

// TelegramPlatformValue stands in for the server's bot token in a credential.
const TelegramPlatformValue = PlatformValuePrefix + "telegram"

// TelegramCredentialTypes are the credential types a Telegram connection fills:
// the Activepieces piece and the hand-written catalog node.
var TelegramCredentialTypes = map[string]bool{"pieces.telegram-bot": true, "telegram": true}

var telegramTokenRe = regexp.MustCompile(`^[0-9]{5,16}:[A-Za-z0-9_-]{30,64}$`)

const telegramConnectTTL = 10 * time.Minute

type tgPending struct {
	owner    domain.MemberId
	credType string
	name     string
	token    string // bot token to poll; the platform token for the server's bot
	platform bool
	bot      string // bot username
	expires  time.Time

	credentialID domain.CredentialId
	chatName     string
	err          string
}

// TelegramConnectService runs deep-link connections. State is in memory, like
// the OAuth flow's: a restart only means starting the connection again.
type TelegramConnectService struct {
	creds         *CredentialService
	log           platform.Logger
	http          *http.Client
	apiBase       string
	platformToken string
	now           func() time.Time

	mu          sync.Mutex
	pending     map[string]*tgPending // code → connection
	polling     map[string]bool       // bot token → poll loop running
	platformBot string                // cached username of the server's bot
}

func NewTelegramConnectService(creds *CredentialService, log platform.Logger, client *http.Client) *TelegramConnectService {
	return &TelegramConnectService{
		creds:         creds,
		log:           log,
		http:          client,
		apiBase:       "https://api.telegram.org",
		platformToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		now:           time.Now,
		pending:       map[string]*tgPending{},
		polling:       map[string]bool{},
	}
}

// TelegramInfo tells the web app which connect options exist.
type TelegramInfo struct {
	PlatformBot bool   `json:"platformBot"`
	BotUsername string `json:"botUsername,omitempty"`
}

func (s *TelegramConnectService) Info(ctx context.Context) TelegramInfo {
	if s.platformToken == "" {
		return TelegramInfo{}
	}
	name, err := s.platformUsername(ctx)
	if err != nil {
		s.log.Warn("telegram platform bot unavailable", "error", err.Error())
		return TelegramInfo{}
	}
	return TelegramInfo{PlatformBot: true, BotUsername: name}
}

// TelegramStart is what the web app shows: the link to open, and the code to
// poll with.
type TelegramStart struct {
	Code        string `json:"code"`
	BotUsername string `json:"botUsername"`
	Link        string `json:"link"`
	ExpiresAt   string `json:"expiresAt"`
}

// Start begins a connection. An empty ownToken uses the server's bot.
func (s *TelegramConnectService) Start(ctx context.Context, owner domain.MemberId, credType, name, ownToken string) (TelegramStart, error) {
	if !TelegramCredentialTypes[credType] {
		return TelegramStart{}, domain.BadRequest("not a Telegram credential type")
	}
	ownToken = strings.TrimSpace(ownToken)
	p := &tgPending{owner: owner, credType: credType, expires: s.now().Add(telegramConnectTTL)}
	if ownToken == "" {
		if s.platformToken == "" {
			return TelegramStart{}, domain.BadRequest("This server has no Telegram bot of its own. Paste your bot's token instead.")
		}
		bot, err := s.platformUsername(ctx)
		if err != nil {
			return TelegramStart{}, domain.Errorf(domain.CodeInternal, "the Apex Telegram bot is unavailable right now")
		}
		p.token, p.platform, p.bot = s.platformToken, true, bot
	} else {
		if !telegramTokenRe.MatchString(ownToken) {
			return TelegramStart{}, domain.BadRequest("That doesn't look like a bot token. It looks like 123456789:AA… and comes from @BotFather.")
		}
		bot, err := s.getMe(ctx, ownToken)
		if err != nil {
			return TelegramStart{}, err
		}
		p.token, p.bot = ownToken, bot
	}
	p.name = strings.TrimSpace(name)

	code := randToken()[:20]
	s.mu.Lock()
	for k, v := range s.pending {
		if s.now().After(v.expires.Add(telegramConnectTTL)) {
			delete(s.pending, k)
		}
	}
	s.pending[code] = p
	start := !s.polling[p.token]
	s.polling[p.token] = true
	s.mu.Unlock()
	if start {
		go s.poll(p.token)
	}
	return TelegramStart{
		Code:        code,
		BotUsername: p.bot,
		Link:        "https://t.me/" + p.bot + "?start=" + code,
		ExpiresAt:   p.expires.UTC().Format(time.RFC3339),
	}, nil
}

// TelegramStatus reports a connection's progress.
type TelegramStatus struct {
	State        string              `json:"state"` // waiting | connected | expired | failed
	CredentialID domain.CredentialId `json:"credentialId,omitempty"`
	ChatName     string              `json:"chatName,omitempty"`
	Error        string              `json:"error,omitempty"`
}

func (s *TelegramConnectService) Status(owner domain.MemberId, code string) TelegramStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[code]
	switch {
	case !ok || p.owner != owner:
		return TelegramStatus{State: "expired"}
	case p.credentialID != "":
		return TelegramStatus{State: "connected", CredentialID: p.credentialID, ChatName: p.chatName}
	case p.err != "":
		return TelegramStatus{State: "failed", Error: p.err}
	case s.now().After(p.expires):
		return TelegramStatus{State: "expired"}
	}
	return TelegramStatus{State: "waiting"}
}

// TelegramChat is a connected chat, so the run dialog can fill a chat id in.
type TelegramChat struct {
	CredentialID domain.CredentialId `json:"credentialId"`
	Name         string              `json:"name"`
	ChatID       string              `json:"chatId"`
	ChatName     string              `json:"chatName,omitempty"`
}

// Chats lists owner's Telegram credentials that know their chat id.
func (s *TelegramConnectService) Chats(owner domain.MemberId) []TelegramChat {
	out := []TelegramChat{}
	for _, c := range s.creds.List(&owner) {
		if !TelegramCredentialTypes[c.Type] {
			continue
		}
		_, data, err := s.creds.Data(&owner, c.ID)
		if err != nil {
			continue
		}
		chatID := fmt.Sprint(data["chat_id"])
		if data["chat_id"] == nil || chatID == "" {
			continue
		}
		name, _ := data["chat_name"].(string)
		out = append(out, TelegramChat{CredentialID: c.ID, Name: c.Name, ChatID: chatID, ChatName: name})
	}
	return out
}

/* ------------------------------ polling ------------------------------- */

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			Title     string `json:"title"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"chat"`
	} `json:"message"`
}

// poll long-polls one bot's updates while it has open connections.
func (s *TelegramConnectService) poll(token string) {
	defer func() {
		s.mu.Lock()
		delete(s.polling, token)
		s.mu.Unlock()
	}()
	var offset int64
	failures := 0
	for s.waitingOn(token) {
		updates, err := s.getUpdates(token, offset)
		if err != nil {
			if strings.Contains(err.Error(), "webhook") || strings.Contains(err.Error(), "409") {
				s.failAll(token, "This bot sends its messages to a webhook, so Apex can't see the Start message. Remove the webhook or use another bot.")
				return
			}
			failures++
			if failures > 20 {
				s.failAll(token, "Apex couldn't reach Telegram. Try again in a minute.")
				return
			}
			time.Sleep(3 * time.Second)
			continue
		}
		failures = 0
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			s.handle(token, u)
		}
	}
}

func (s *TelegramConnectService) waitingOn(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pending {
		if p.token == token && p.credentialID == "" && p.err == "" && s.now().Before(p.expires) {
			return true
		}
	}
	return false
}

func (s *TelegramConnectService) failAll(token, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pending {
		if p.token == token && p.credentialID == "" {
			p.err = msg
		}
	}
}

func (s *TelegramConnectService) handle(token string, u tgUpdate) {
	if u.Message == nil {
		return
	}
	cmd, arg, _ := strings.Cut(strings.TrimSpace(u.Message.Text), " ")
	if cmd != "/start" && !strings.HasPrefix(cmd, "/start@") {
		return
	}
	code := strings.TrimSpace(arg)
	chat := u.Message.Chat

	s.mu.Lock()
	p, ok := s.pending[code]
	usable := ok && p.token == token && p.credentialID == "" && p.err == "" && s.now().Before(p.expires)
	s.mu.Unlock()
	if !usable {
		if code != "" || chat.Type == "private" {
			s.send(token, chat.ID, "This link has expired. Go back to Apex and click Connect Telegram again.")
		}
		return
	}

	chatName := strings.TrimSpace(chat.FirstName + " " + chat.LastName)
	if chat.Title != "" {
		chatName = chat.Title
	}
	if chat.Username != "" {
		chatName = "@" + chat.Username
	}
	name := p.name
	if name == "" {
		name = "Telegram · " + chatName
	}
	secret := p.token
	if p.platform {
		secret = TelegramPlatformValue
	}
	chatID := strconv.FormatInt(chat.ID, 10)
	summary, err := s.creds.CreateConnected(&p.owner, name, p.credType, map[string]any{
		"value":              secret,
		"TELEGRAM_BOT_TOKEN": secret,
		"chat_id":            chatID,
		"chat_name":          chatName,
		"bot_username":       p.bot,
	})

	s.mu.Lock()
	if err != nil {
		p.err = "Apex couldn't save the connection. Try again."
	} else {
		p.credentialID, p.chatName = summary.ID, chatName
	}
	s.mu.Unlock()
	if err != nil {
		s.log.Error("telegram connect save failed", "error", err.Error())
		return
	}
	s.send(token, chat.ID, "✅ Connected to Apex. Your workflows can now send messages to this chat.")
}

/* ---------------------------- Bot API calls ---------------------------- */

type tgResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
}

func (s *TelegramConnectService) call(ctx context.Context, token, method string, params url.Values) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBase+"/bot"+token+"/"+method, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		// The error text holds the URL, and so the token: never pass it on.
		return nil, fmt.Errorf("telegram %s: request failed", method)
	}
	defer resp.Body.Close()
	var out tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("telegram %s: unreadable response (status %d)", method, resp.StatusCode)
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram %s: %d %s", method, out.ErrorCode, out.Description)
	}
	return out.Result, nil
}

func (s *TelegramConnectService) getMe(ctx context.Context, token string) (string, error) {
	raw, err := s.call(ctx, token, "getMe", url.Values{})
	if err != nil {
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "404") {
			return "", domain.BadRequest("Telegram doesn't recognise that bot token. Copy it again from @BotFather.")
		}
		return "", domain.Errorf(domain.CodeInternal, "couldn't reach Telegram")
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(raw, &me); err != nil || me.Username == "" {
		return "", domain.Errorf(domain.CodeInternal, "Telegram returned no bot username")
	}
	return me.Username, nil
}

func (s *TelegramConnectService) platformUsername(ctx context.Context) (string, error) {
	s.mu.Lock()
	cached := s.platformBot
	s.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	name, err := s.getMe(ctx, s.platformToken)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.platformBot = name
	s.mu.Unlock()
	return name, nil
}

func (s *TelegramConnectService) getUpdates(token string, offset int64) ([]tgUpdate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	raw, err := s.call(ctx, token, "getUpdates", url.Values{
		"offset":          {strconv.FormatInt(offset, 10)},
		"timeout":         {"25"},
		"allowed_updates": {`["message"]`},
	})
	if err != nil {
		return nil, err
	}
	var updates []tgUpdate
	if err := json.Unmarshal(raw, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (s *TelegramConnectService) send(token string, chatID int64, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.call(ctx, token, "sendMessage", url.Values{
		"chat_id": {strconv.FormatInt(chatID, 10)},
		"text":    {text},
	}); err != nil {
		s.log.Warn("telegram reply failed", "error", err.Error())
	}
}
