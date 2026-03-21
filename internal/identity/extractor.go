// @forge-project: Forge
// @forge-path: internal/identity/extractor.go
// Package identity provides actor identity extraction for Forge execution recording.
// ADR-042: every Forge execution records who triggered it.
//
// Extraction is fail-open — if Gate is unreachable or no token is present,
// the execution proceeds with an empty actor. Guardian G-009 detects this.
package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	canon "github.com/Harshmaury/Canon/identity"
)

const validateTimeout = 2 * time.Second

// Actor holds the extracted identity for one execution.
// Empty Subject means anonymous — no identity token was present or valid.
type Actor struct {
	Subject string   // Gate sub claim — e.g. "harsh@github" or "agent:local"
	Scopes  []string // Gate scp claim
}

// Empty returns true if no identity was extracted.
func (a Actor) Empty() bool { return a.Subject == "" }

// Extractor extracts actor identity from HTTP requests via Gate validation.
type Extractor struct {
	gateAddr   string
	httpClient *http.Client
}

// NewExtractor creates an Extractor.
// gateAddr is the Gate service address (e.g. http://127.0.0.1:8088).
func NewExtractor(gateAddr string) *Extractor {
	return &Extractor{
		gateAddr:   gateAddr,
		httpClient: &http.Client{Timeout: validateTimeout},
	}
}

// Extract reads X-Identity-Token from the request and validates it with Gate.
// Returns an empty Actor (not an error) if the token is absent or Gate is down.
// Fail-open: execution always proceeds — Guardian G-009 flags anonymous executions.
func (e *Extractor) Extract(ctx context.Context, r *http.Request) Actor {
	token := r.Header.Get(canon.IdentityTokenHeader)
	if token == "" {
		return Actor{}
	}
	claim, err := e.validate(ctx, token)
	if err != nil {
		return Actor{} // Gate unreachable — fail open
	}
	if !claim.valid {
		return Actor{} // invalid token — fail open
	}
	return Actor{Subject: claim.subject, Scopes: claim.scopes}
}

type validationResult struct {
	valid   bool
	subject string
	scopes  []string
}

func (e *Extractor) validate(ctx context.Context, token string) (validationResult, error) {
	body := fmt.Sprintf(`{"token":%q}`, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.gateAddr+"/gate/validate", strings.NewReader(body))
	if err != nil {
		return validationResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return validationResult{}, err
	}
	defer resp.Body.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Valid  bool   `json:"valid"`
			Reason string `json:"reason,omitempty"`
			Claim  *struct {
				Subject string   `json:"sub"`
				Scopes  []string `json:"scp"`
			} `json:"claim,omitempty"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return validationResult{}, err
	}
	if !envelope.OK || !envelope.Data.Valid || envelope.Data.Claim == nil {
		return validationResult{valid: false}, nil
	}
	return validationResult{
		valid:   true,
		subject: envelope.Data.Claim.Subject,
		scopes:  envelope.Data.Claim.Scopes,
	}, nil
}
