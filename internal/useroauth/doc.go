// Package useroauth acquires user-scoped Google OAuth access tokens for workspace
// datastore ACLs. Refresh tokens are stored after a one-time consent flow; each
// /prompt request mints a short-lived access token server-side. See docs/architecture.md.
package useroauth
