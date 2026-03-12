package server

func (s *Server) boundAccountEmail(accountID string) string {
	if accountID == "" {
		return ""
	}
	acct := s.pool.Get(accountID)
	if acct == nil {
		return ""
	}
	return acct.Email
}
