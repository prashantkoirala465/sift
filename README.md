# Sift

A self-hosted job application tracker that updates itself. Sift watches your
Gmail inbox, classifies incoming mail against the applications you're
tracking (rejection, interview invite, offer, assessment request, ...), and
keeps your pipeline current without you copy-pasting status updates into a
spreadsheet.

**Status**: early development. Not yet usable.

## The problem

Every application tracker — a spreadsheet, Notion, a paid tool — goes stale
the same way: the data only updates when a human remembers to update it, and
after a few dozen applications nobody does. Sift instead treats your inbox as
the source of truth and reconciles your tracked applications against it.

## Design

- Single Go binary, server-rendered UI (htmx, no SPA build step), Postgres
  for storage. Runs anywhere `docker run` works.
- Single-user, self-hosted. No multi-tenant complexity that a personal tool
  doesn't need.
- Email classification starts with sender-domain and keyword heuristics; an
  LLM is only consulted for mail the rules can't confidently place, and the
  result is cached.
- Anything the matcher isn't confident about goes into a manual review queue
  rather than being silently (and possibly wrongly) auto-linked.
- Reads only what it needs from your inbox, least-privilege OAuth scopes,
  encrypts stored tokens at rest.

Architecture and setup instructions will land here as the pieces exist.

## License

MIT — see [LICENSE](LICENSE).
