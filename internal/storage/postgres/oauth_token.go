package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/prashantkoirala465/sift/internal/gmail"
	"github.com/prashantkoirala465/sift/internal/security"
	"github.com/prashantkoirala465/sift/internal/storage/postgres/sqlc"
)

// SaveToken and LoadToken make *Store satisfy gmail.TokenStore. Access and
// refresh tokens are encrypted before they ever reach the database --
// anyone with read access to a Postgres dump still can't act as the user's
// Gmail account without the encryption key, which lives only in the
// process environment.

func (s *Store) SaveToken(ctx context.Context, token *oauth2.Token) error {
	accessEnc, err := security.Encrypt(s.encryptionKey, []byte(token.AccessToken))
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	refreshEnc, err := security.Encrypt(s.encryptionKey, []byte(token.RefreshToken))
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	err = s.q.UpsertOAuthToken(ctx, sqlc.UpsertOAuthTokenParams{
		Provider:              "google",
		AccessTokenEncrypted:  accessEnc,
		RefreshTokenEncrypted: refreshEnc,
		TokenType:             token.TokenType,
		Expiry:                pgTimestamptz(token.Expiry),
	})
	if err != nil {
		return fmt.Errorf("save oauth token: %w", err)
	}
	return nil
}

func (s *Store) LoadToken(ctx context.Context) (*oauth2.Token, error) {
	row, err := s.q.GetOAuthToken(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, gmail.ErrNoToken
		}
		return nil, fmt.Errorf("load oauth token: %w", err)
	}

	access, err := security.Decrypt(s.encryptionKey, row.AccessTokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := security.Decrypt(s.encryptionKey, row.RefreshTokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	return &oauth2.Token{
		AccessToken:  string(access),
		RefreshToken: string(refresh),
		TokenType:    row.TokenType,
		Expiry:       row.Expiry.Time,
	}, nil
}
