package useroauth

import "errors"

// ErrConsentRequired indicates the user has not completed Google OAuth consent
// and has no stored refresh token for workspace datastore access.
var ErrConsentRequired = errors.New("google oauth consent required")
