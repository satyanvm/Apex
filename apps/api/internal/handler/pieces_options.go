package handler

// pieces_options.go proxies dynamic-prop options lookups to the pieces
// worker's HTTP resolver (apps/pieces-worker/src/options-server.ts), which
// invokes the piece prop's real options() with the node's credential. The
// worker being down is a business outcome for the builder UI (it falls back
// to By-ID entry), so it maps to {ok:false} — never a 5xx.
//
//	POST /api/pieces/options  → forwarded verbatim; response passed through.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
)

const piecesOptionsMaxBody = 1 << 20

func piecesOptions(p *services.Platform) http.HandlerFunc {
	client := &http.Client{Timeout: 12 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		base := os.Getenv("PIECES_WORKER_URL")
		if base == "" {
			base = "http://localhost:7391"
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, piecesOptionsMaxBody))
		if err != nil {
			writeOptionsError(w, "unreadable request body", "BadRequest")
			return
		}
		// The worker looks options up with the named credential, or with an env
		// var of the server's. Only the caller's own credential is allowed.
		var lookup struct {
			CredentialID string `json:"credentialId"`
			AuthEnvKey   string `json:"authEnvKey"`
		}
		_ = json.Unmarshal(body, &lookup)
		if lookup.AuthEnvKey != "" && lookup.CredentialID == "" {
			writeOptionsError(w, "connect your own account to load these options", "MissingCredential")
			return
		}
		if lookup.CredentialID != "" {
			if err := p.Credentials.CheckOwned(domain.CredentialId(lookup.CredentialID), actorID(r)); err != nil {
				writeOptionsError(w, "credential not found", "MissingCredential")
				return
			}
			// Drop the env fallback so a missing credential can't fall through to it.
			var raw map[string]any
			if json.Unmarshal(body, &raw) == nil {
				delete(raw, "authEnvKey")
				if clean, err := json.Marshal(raw); err == nil {
					body = clean
				}
			}
		}
		resp, err := client.Post(base+"/options", "application/json", bytes.NewReader(body))
		if err != nil {
			writeOptionsError(w, "pieces worker unavailable", "OptionsUnavailable")
			return
		}
		defer resp.Body.Close()
		out, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			writeOptionsError(w, "pieces worker response unreadable", "OptionsUnavailable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(out)
	}
}

func writeOptionsError(w http.ResponseWriter, msg, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":false,"error":"` + msg + `","errorType":"` + errType + `"}`))
}
