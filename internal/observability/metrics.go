// Package observability provides operational visibility sized for a
// single-user, self-hosted tool: stdlib expvar counters exposed as JSON
// at /debug/vars, plus a request-logging middleware. Not a
// Prometheus/Grafana stack -- that tradeoff is right for a multi-tenant
// gateway serving other people's traffic, not for one person's inbox and
// job pipeline, where anyone can already `curl localhost:8080/debug/vars`
// and get an answer.
package observability

import "expvar"

var (
	SyncTicksTotal      = expvar.NewInt("sync_ticks_total")
	SyncTickErrorsTotal = expvar.NewInt("sync_tick_errors_total")
	EmailsSeenTotal     = expvar.NewInt("emails_seen_total")
	EmailsIngestedTotal = expvar.NewInt("emails_ingested_total")

	// ClassificationsBySource counts classifications keyed by which tier
	// produced them ("rule" or "llm") -- the metric that answers "is the
	// LLM fallback actually being consulted rarely, like it's supposed
	// to be, or is the rule classifier missing everything."
	ClassificationsBySource = expvar.NewMap("classifications_by_source_total")

	// StageTransitionsByVia counts pipeline stage transitions keyed by
	// domain.DetectedVia ("email_auto" or "manual").
	StageTransitionsByVia = expvar.NewMap("stage_transitions_by_detected_via_total")

	// HTTPRequestsByStatusClass counts responses keyed by status class
	// ("2xx", "4xx", "5xx", ...).
	HTTPRequestsByStatusClass = expvar.NewMap("http_requests_by_status_class_total")
)
