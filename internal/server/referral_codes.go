package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/yansircc/llm-broker/internal/domain"
)

const (
	referralCodeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	referralCodeLength   = 6
	referralCodeAttempts = 16
)

var (
	errReferralCodeEntropy     = errors.New("referral code entropy unavailable")
	errReferralCodeUnavailable = errors.New("referral code unavailable")
)

func (s *Server) createUserWithReferralCode(ctx context.Context, u *domain.User) error {
	if u == nil {
		return fmt.Errorf("missing user")
	}
	var lastErr error
	for range referralCodeAttempts {
		code, err := generateReferralCode()
		if err != nil {
			return err
		}
		u.ReferralCode = code
		err = s.store.CreateUser(ctx, u)
		if err == nil {
			return nil
		}
		lastErr = err
		existing, lookupErr := s.store.GetUserByReferralCode(ctx, code)
		if lookupErr == nil && existing != nil {
			continue
		}
		return err
	}
	return fmt.Errorf("%w: %v", errReferralCodeUnavailable, lastErr)
}

func generateReferralCode() (string, error) {
	out := make([]byte, referralCodeLength)
	max := big.NewInt(int64(len(referralCodeAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("%w: %v", errReferralCodeEntropy, err)
		}
		out[i] = referralCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

func isReferralCodeAllocationError(err error) bool {
	return errors.Is(err, errReferralCodeEntropy) || errors.Is(err, errReferralCodeUnavailable)
}
