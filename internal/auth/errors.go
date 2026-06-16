package auth

import "errors"

var (
	ErrNoCredentials       = errors.New("no credentials provided")
	ErrVerificationFailed  = errors.New("verification failed")
)
