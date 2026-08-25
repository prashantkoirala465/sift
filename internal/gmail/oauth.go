// Package gmail handles everything Gmail-specific: OAuth, the API client,
// and (starting with the sync worker) reading mail. Nothing outside this
// package should import golang.org/x/oauth2 or google.golang.org/api
// directly.
package gmail

import (
	"context"
	"errors"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ReadOnlyScope is the only scope Sift ever requests. It cannot send,
// delete, or modify anything in the user's mailbox.
const ReadOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"

func NewOAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{ReadOnlyScope},
		Endpoint:     google.Endpoint,
	}
}

// ErrNoToken is returned by a TokenStore when no token has been saved yet --
// i.e. the user hasn't connected a Gmail account.
var ErrNoToken = errors.New("gmail: no stored token")

// TokenStore persists the single OAuth token Sift holds. Defined here, not
// in the storage package, so this package stays ignorant of Postgres; any
// storage backend that can save and load one token satisfies it.
type TokenStore interface {
	SaveToken(ctx context.Context, token *oauth2.Token) error
	LoadToken(ctx context.Context) (*oauth2.Token, error)
}
