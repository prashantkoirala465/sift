package gmail

import (
	"errors"

	"google.golang.org/api/googleapi"
)

// ErrHistoryExpired means the stored history checkpoint is older than
// Gmail's retention window (Google prunes history after roughly a week of
// inactivity). The caller must fall back to a fresh backfill; there is no
// way to recover the missed window from the History API.
var ErrHistoryExpired = errors.New("gmail: history checkpoint expired, backfill required")

func isHistoryExpired(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 404
}
