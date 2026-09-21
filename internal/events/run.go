package events

import (
	"context"
	"log/slog"
)

// Run derives the events from the database and writes them back: the
// `events` stage of cmd/ingest. Idempotent; a second run over the same rows
// changes nothing.
func Run(ctx context.Context, store *Store, overrides *Overrides, log *slog.Logger) (Result, Stats, error) {
	rows, err := store.Load(ctx)
	if err != nil {
		return Result{}, Stats{}, err
	}
	res := Resolve(rows, overrides)
	stats, err := store.Write(ctx, res.Events)
	if err != nil {
		return res, stats, err
	}

	log.InfoContext(ctx, "events derived",
		"rows", stats.Rows, "events", stats.Events, "created", stats.Created, "removed", stats.Removed,
		"by_number", res.ByLink[LinkNumber], "by_override", res.ByLink[LinkOverride],
		"bridged", res.ByLink[LinkBridged], "by_name", res.ByLink[LinkName], "team_ties", res.ByLink[LinkTeam],
		"numbered_within_season", res.Numbered, "names_under_several_numbers", len(res.Ambiguous))
	// Both are decisions a person should look at: an override a source has
	// since undone, and a name the rule declined to bridge.
	for _, ov := range res.Unmatched {
		log.WarnContext(ctx, "event override filed no row", "tour", ov.Tour,
			"number", ov.Number, "code", ov.Code, "source_id", ov.SourceID, "to", ov.To, "note", ov.Note)
	}
	for _, a := range res.Ambiguous {
		log.DebugContext(ctx, "name under several numbers, kept apart", "name", a)
	}
	return res, stats, nil
}
