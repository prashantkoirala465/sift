package gmail

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// persistingTokenSource wraps the oauth2 refresh flow and writes the token
// back to the store whenever it changes. Without this, a refreshed access
// token only ever lives in memory and every restart re-triggers a refresh
// against Google using the same still-valid refresh token -- harmless, but
// it also means a revoked-and-reissued refresh token would never actually
// get persisted.
type persistingTokenSource struct {
	ctx   context.Context
	base  oauth2.TokenSource
	store TokenStore
	last  string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.last {
		if err := p.store.SaveToken(p.ctx, tok); err != nil {
			return nil, fmt.Errorf("persist refreshed token: %w", err)
		}
		p.last = tok.AccessToken
	}
	return tok, nil
}

// HTTPClient returns a client authorized against the stored token. Expired
// tokens are refreshed transparently and the refreshed token is persisted
// via store.
func HTTPClient(ctx context.Context, cfg *oauth2.Config, store TokenStore) (*http.Client, error) {
	tok, err := store.LoadToken(ctx)
	if err != nil {
		return nil, err
	}

	base := cfg.TokenSource(ctx, tok)
	src := oauth2.ReuseTokenSource(tok, &persistingTokenSource{
		ctx:   ctx,
		base:  base,
		store: store,
		last:  tok.AccessToken,
	})

	return oauth2.NewClient(ctx, src), nil
}
