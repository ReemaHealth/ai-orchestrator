package auth

import "errors"

// Authentication errors returned by verify functions. Middleware maps these to HTTP status:
// ErrNoCredentials → 401, ErrVerificationFailed → 403.
var (
	ErrNoCredentials       = errors.New("no credentials provided")
	ErrVerificationFailed  = errors.New("verification failed")
)
