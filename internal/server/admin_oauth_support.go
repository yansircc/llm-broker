package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	CellID   string `json:"cell_id,omitempty"`
}

type generateAuthURLRequest struct {
	Provider string `json:"provider"`
	CellID   string `json:"cell_id"`
}

type exchangeCodeRequest struct {
	Provider     string `json:"provider"`
	SessionID    string `json:"session_id"`
	CellID       string `json:"cell_id"`
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

func decodeGenerateAuthURLRequest(r *http.Request) (*generateAuthURLRequest, error) {
	req := &generateAuthURLRequest{
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		CellID:   strings.TrimSpace(r.URL.Query().Get("cell_id")),
	}
	if r.Body == nil {
		return req, nil
	}

	var bodyReq generateAuthURLRequest
	if err := json.NewDecoder(r.Body).Decode(&bodyReq); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if provider := strings.TrimSpace(bodyReq.Provider); provider != "" {
		req.Provider = provider
	}
	if cellID := strings.TrimSpace(bodyReq.CellID); cellID != "" {
		req.CellID = cellID
	}
	return req, nil
}

func (s *Server) storeOAuthSession(ctx context.Context, provider, cellID string, session driver.OAuthSession) (string, error) {
	sessionID := uuid.New().String()
	payload, err := json.Marshal(oauthSessionEnvelope{
		OAuthSession: session,
		Provider:     provider,
		CellID:       cellID,
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
		sessionJSON, ok, err := s.pool.GetOAuthSession(ctx, req.SessionID)
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
		if req.CellID == "" && session.CellID != "" {
			req.CellID = session.CellID
		}
		if req.CallbackURL != "" && req.Code == "" {
			req.Code = extractCodeFromCallback(req.CallbackURL)
		}
	}
	req.Provider = strings.TrimSpace(req.Provider)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.CellID = strings.TrimSpace(req.CellID)
	req.CallbackURL = strings.TrimSpace(req.CallbackURL)
	req.CodeVerifier = strings.TrimSpace(req.CodeVerifier)
	req.State = strings.TrimSpace(req.State)
	if req.Code != "" {
		req.Code = extractCodeFromCallback(req.Code)
	}
	return nil
}

func (s *Server) validateExchangeCellSelection(existing *domain.Account, requestedCellID string, provider domain.Provider) error {
	requestedCellID = strings.TrimSpace(requestedCellID)
	if requestedCellID == "" {
		return fmt.Errorf("cell_id is required")
	}

	currentAccountID := ""
	currentCellID := ""
	if existing != nil {
		currentAccountID = existing.ID
		currentCellID = existing.CellID
	}
	if requestedCellID == currentCellID {
		return nil
	}

	cell := s.pool.GetCell(requestedCellID)
	if reason := accountCellBindError(cell, time.Now().UTC()); reason != "" {
		return fmt.Errorf("%s", reason)
	}
	if accountOwnsCell(s.pool.List(), currentAccountID, requestedCellID, provider) {
		return fmt.Errorf("cell is already bound to another account of the same provider")
	}
	return nil
}

func (s *Server) oauthClientForCell(cellID string) (*http.Client, error) {
	cellID = strings.TrimSpace(cellID)
	if cellID == "" {
		return nil, fmt.Errorf("cell_id is required")
	}

	cell := s.pool.GetCell(cellID)
	if reason := accountCellBindError(cell, time.Now().UTC()); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	if s.transportPool == nil {
		return nil, fmt.Errorf("oauth transport is unavailable")
	}

	return s.transportPool.ClientForAccount(&domain.Account{
		CellID: cellID,
		Cell:   cell,
	}), nil
}

func (s *Server) upsertExchangedAccount(drv driver.OAuthDriver, result *driver.ExchangeResult, cellID string) (exchangeAccountResponse, error) {
	if existing := s.pool.FindBySubject(drv.Provider(), result.Subject); existing != nil {
		return s.updateExchangedAccount(existing, result, cellID)
	}
	return s.createExchangedAccount(drv, result, cellID)
}

func (s *Server) updateExchangedAccount(existing *domain.Account, result *driver.ExchangeResult, cellID string) (exchangeAccountResponse, error) {
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
		a.CellID = cellID
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

func (s *Server) createExchangedAccount(drv driver.OAuthDriver, result *driver.ExchangeResult, cellID string) (exchangeAccountResponse, error) {
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
		CellID:          cellID,
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
