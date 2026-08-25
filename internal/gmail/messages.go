package gmail

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Message is the slice of a Gmail message Sift actually needs. Notably not
// the full body -- Snippet (Gmail's own short plain-text preview, returned
// even at format=metadata) gives the classifier a bit more than the
// subject line without Sift ever having to parse multipart MIME.
type Message struct {
	ID         string
	ThreadID   string
	From       string
	FromDomain string
	Subject    string
	Snippet    string
	ReceivedAt time.Time
}

type Service struct {
	api *gmailapi.Service
}

func NewService(ctx context.Context, httpClient *http.Client) (*Service, error) {
	svc, err := gmailapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Service{api: svc}, nil
}

// CurrentHistoryID returns the mailbox's history ID as of now -- the
// checkpoint an incremental sync should resume from.
func (s *Service) CurrentHistoryID(ctx context.Context) (uint64, error) {
	profile, err := s.api.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("get profile: %w", err)
	}
	return profile.HistoryId, nil
}

// ListRecent lists message IDs matching a Gmail search query, for the
// initial backfill. Stops once maxResults IDs have been collected.
func (s *Service) ListRecent(ctx context.Context, query string, maxResults int64) ([]string, error) {
	var ids []string
	pageToken := ""

	for {
		call := s.api.Users.Messages.List("me").Q(query).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}

		for _, m := range resp.Messages {
			ids = append(ids, m.Id)
			if int64(len(ids)) >= maxResults {
				return ids, nil
			}
		}

		if resp.NextPageToken == "" {
			return ids, nil
		}
		pageToken = resp.NextPageToken
	}
}

// ListHistorySince returns message IDs added since historyID via the
// incremental History API, plus the new checkpoint to persist. If Gmail
// reports the starting history ID is too old (it prunes history after
// roughly a week), ErrHistoryExpired is returned -- the caller should fall
// back to a fresh backfill.
func (s *Service) ListHistorySince(ctx context.Context, historyID uint64) ([]string, uint64, error) {
	var ids []string
	newHistoryID := historyID
	pageToken := ""

	for {
		call := s.api.Users.History.List("me").
			StartHistoryId(historyID).
			HistoryTypes("messageAdded").
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			if isHistoryExpired(err) {
				return nil, 0, ErrHistoryExpired
			}
			return nil, 0, fmt.Errorf("list history: %w", err)
		}

		for _, h := range resp.History {
			for _, added := range h.MessagesAdded {
				ids = append(ids, added.Message.Id)
			}
		}
		if resp.HistoryId > newHistoryID {
			newHistoryID = resp.HistoryId
		}

		if resp.NextPageToken == "" {
			return ids, newHistoryID, nil
		}
		pageToken = resp.NextPageToken
	}
}

// GetMessage fetches headers and receipt time for one message.
// format=metadata keeps the payload small; nothing built so far needs the
// message body.
func (s *Service) GetMessage(ctx context.Context, id string) (Message, error) {
	msg, err := s.api.Users.Messages.Get("me", id).
		Format("metadata").
		MetadataHeaders("From", "Subject").
		Context(ctx).
		Do()
	if err != nil {
		return Message{}, fmt.Errorf("get message %s: %w", id, err)
	}

	m := Message{ID: msg.Id, ThreadID: msg.ThreadId, Snippet: msg.Snippet, ReceivedAt: time.UnixMilli(msg.InternalDate)}
	if msg.Payload != nil {
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "From":
				m.From, m.FromDomain = parseFromHeader(h.Value)
			case "Subject":
				m.Subject = h.Value
			}
		}
	}

	return m, nil
}

// parseFromHeader pulls the bare address and lowercased domain out of a
// From header, which arrives as either "Name <user@domain.com>" or a bare
// "user@domain.com".
func parseFromHeader(v string) (address, domain string) {
	v = strings.TrimSpace(v)
	if i := strings.LastIndex(v, "<"); i != -1 {
		if j := strings.Index(v[i:], ">"); j != -1 {
			v = v[i+1 : i+j]
		}
	}
	v = strings.TrimSpace(v)

	if at := strings.LastIndex(v, "@"); at != -1 {
		return v, strings.ToLower(v[at+1:])
	}
	return v, ""
}
