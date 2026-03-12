package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

var (
	errInvalidOAuthSession = errors.New("invalid or expired session_id")
	errCorruptOAuthSession = errors.New("corrupt session data")
)

type oauthSessionEnvelope struct {
	driver.OAuthSession
	Provider string `json:"provider"`
}

type exchangeCodeRequest struct {
	Provider     string `json:"provider"`
	SessionID    string `json:"session_id"`
	CallbackURL  string `json:"callback_url"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
}

type exchangeAccountResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

func (s *Server) storeOAuthSession(ctx context.Context, provider string, session driver.OAuthSession) (string, error) {
	sessionID := uuid.New().String()
	payload, err := json.Marshal(oauthSessionEnvelope{
		OAuthSession: session,
		Provider:     provider,
	})
	if err != nil {
		return "", err
	}
	if err := s.pool.SetOAuthSession(ctx, sessionID, string(payload), 10*time.Minute); err != nil {
		return "", err
	}
	return sessionID, nil
}

func decodeExchangeCodeRequest(r *http.Request) (*exchangeCodeRequest, error) {
	var req exchangeCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func (s *Server) hydrateExchangeCodeRequest(ctx context.Context, req *exchangeCodeRequest) error {
	if req.SessionID != "" {
		sessionJSON, ok, err := s.pool.GetDelOAuthSession(ctx, req.SessionID)
		if err != nil {
			return err
		}
		if !ok {
			return errInvalidOAuthSession
		}

		var session oauthSessionEnvelope
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			return errCorruptOAuthSession
		}

		req.CodeVerifier = session.CodeVerifier
		req.State = session.State
		if session.Provider != "" {
			req.Provider = session.Provider
		}
		if req.CallbackURL != "" && req.Code == "" {
			req.Code = extractCodeFromCallback(req.CallbackURL)
		}
	}
	if req.Code != "" {
		req.Code = extractCodeFromCallback(req.Code)
	}
	return nil
}

func (s *Server) upsertExchangedAccount(drv driver.OAuthDriver, result *driver.ExchangeResult) (exchangeAccountResponse, error) {
	if existing := s.pool.FindBySubject(drv.Provider(), result.Subject); existing != nil {
		return s.updateExchangedAccount(existing, result)
	}
	return s.createExchangedAccount(drv, result)
}

func (s *Server) updateExchangedAccount(existing *domain.Account, result *driver.ExchangeResult) (exchangeAccountResponse, error) {
	encAccess, encRefresh, err := s.encryptExchangeTokens(result.AccessToken, result.RefreshToken, existing.RefreshTokenEnc)
	if err != nil {
		return exchangeAccountResponse{}, err
	}

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UnixMilli()
	if err := s.pool.StoreTokens(existing.ID, encAccess, encRefresh, expiresAt); err != nil {
		return exchangeAccountResponse{}, err
	}

	_ = s.pool.Update(existing.ID, func(a *domain.Account) {
		a.Email = result.Email
		a.Status = domain.StatusActive
		a.Identity = result.Identity
		a.Subject = result.Subject
		if len(result.ProviderState) > 0 {
			a.ProviderStateJSON = string(result.ProviderState)
		}
	})

	return exchangeAccountResponse{
		ID:     existing.ID,
		Email:  result.Email,
		Status: string(domain.StatusActive),
	}, nil
}

func (s *Server) createExchangedAccount(drv driver.OAuthDriver, result *driver.ExchangeResult) (exchangeAccountResponse, error) {
	encAccess, encRefresh, err := s.encryptExchangeTokens(result.AccessToken, result.RefreshToken, "")
	if err != nil {
		return exchangeAccountResponse{}, err
	}

	now := time.Now().UTC()
	acct := &domain.Account{
		ID:              uuid.New().String(),
		Email:           result.Email,
		Provider:        drv.Provider(),
		Subject:         result.Subject,
		Status:          domain.StatusActive,
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: encRefresh,
		AccessTokenEnc:  encAccess,
		ExpiresAt:       now.Add(time.Duration(result.ExpiresIn) * time.Second).UnixMilli(),
		CreatedAt:       now,
		Identity:        result.Identity,
		ProviderStateJSON: func() string {
			if len(result.ProviderState) == 0 {
				return "{}"
			}
			return string(result.ProviderState)
		}(),
		LastRefreshAt: &now,
	}
	acct.BucketKey = drv.BucketKey(acct)

	if err := s.pool.Add(acct); err != nil {
		return exchangeAccountResponse{}, err
	}

	return exchangeAccountResponse{
		ID:     acct.ID,
		Email:  result.Email,
		Status: string(domain.StatusActive),
	}, nil
}

func (s *Server) encryptExchangeTokens(accessToken, refreshToken, fallbackRefreshEnc string) (string, string, error) {
	encAccess, err := s.tokens.EncryptToken(accessToken)
	if err != nil {
		return "", "", err
	}

	encRefresh := fallbackRefreshEnc
	if refreshToken != "" || encRefresh == "" {
		encRefresh, err = s.tokens.EncryptToken(refreshToken)
		if err != nil {
			return "", "", err
		}
	}

	return encAccess, encRefresh, nil
}

func extractCodeFromCallback(callbackURL string) string {
	s := strings.TrimSpace(callbackURL)
	if s == "" {
		return ""
	}

	parsed, err := url.Parse(s)
	if err != nil || parsed.Scheme == "" {
		if i := strings.Index(s, "#"); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "&"); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "?"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimPrefix(s, "code=")
		return strings.TrimSpace(s)
	}
	if code := parsed.Query().Get("code"); code != "" {
		return code
	}
	return strings.TrimSpace(s)
}
