package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"runloop/internal/sources"
)

const (
	Type              = "schedule"
	defaultEntityType = "schedule_tick"
	maxCatchUp        = 100
)

func init() {
	sources.Register(Type, func(id string, cfg map[string]any, opts sources.BuildOptions) (sources.Source, error) {
		return New(id, cfg)
	})
}

type Schedule interface {
	Next(after time.Time) time.Time
}

type intervalSchedule struct{ d time.Duration }

func (i intervalSchedule) Next(after time.Time) time.Time { return after.Add(i.d) }

type cronSchedule struct{ s cron.Schedule }

func (c cronSchedule) Next(after time.Time) time.Time { return c.s.Next(after) }

type Source struct {
	id         string
	schedule   Schedule
	entityType string
	payload    map[string]any
	now        func() time.Time
}

func New(id string, cfg map[string]any) (*Source, error) {
	sched, err := parseSchedule(id, cfg)
	if err != nil {
		return nil, err
	}
	entityType, _ := cfg["entityType"].(string)
	if entityType == "" {
		entityType = defaultEntityType
	}
	payload, _ := cfg["payload"].(map[string]any)
	return &Source{id: id, schedule: sched, entityType: entityType, payload: payload, now: time.Now}, nil
}

func parseSchedule(id string, cfg map[string]any) (Schedule, error) {
	everyVal, hasEvery := cfg["every"]
	cronVal, hasCron := cfg["cron"]
	if hasEvery && hasCron {
		return nil, fmt.Errorf("schedule source %q: set exactly one of every / cron", id)
	}
	if !hasEvery && !hasCron {
		return nil, fmt.Errorf("schedule source %q: requires every or cron", id)
	}
	if hasEvery {
		raw, ok := everyVal.(string)
		if !ok {
			return nil, fmt.Errorf("schedule source %q: every must be a duration string", id)
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("schedule source %q: invalid every %q: %w", id, raw, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("schedule source %q: every must be positive", id)
		}
		return intervalSchedule{d: d}, nil
	}
	expr, ok := cronVal.(string)
	if !ok {
		return nil, fmt.Errorf("schedule source %q: cron must be a string expression", id)
	}
	parsed, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, fmt.Errorf("schedule source %q: invalid cron %q: %w", id, expr, err)
	}
	return cronSchedule{s: parsed}, nil
}

func (s *Source) ID() string   { return s.id }
func (s *Source) Type() string { return Type }

func (s *Source) Test(ctx context.Context) error {
	return ctx.Err()
}

func (s *Source) Sync(ctx context.Context, cursor sources.Cursor) ([]sources.InboxCandidate, sources.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, cursor, err
	}
	now := s.now().UTC()
	if cursor.IsZero() {
		// First sync establishes a baseline so the source does not back-fire on startup.
		return nil, sources.Cursor{Value: now.Format(time.RFC3339Nano)}, nil
	}
	last, err := time.Parse(time.RFC3339Nano, cursor.Value)
	if err != nil {
		return nil, cursor, fmt.Errorf("schedule source %q: invalid cursor %q: %w", s.id, cursor.Value, err)
	}
	last = last.UTC()

	var candidates []sources.InboxCandidate
	cursorTime := last
	for i := 0; i < maxCatchUp; i++ {
		next := s.schedule.Next(cursorTime)
		if next.After(now) {
			break
		}
		fired := next.UTC()
		payload := map[string]any{}
		for k, v := range s.payload {
			payload[k] = v
		}
		payload["firedAt"] = fired.Format(time.RFC3339Nano)
		payload["sourceId"] = s.id
		candidates = append(candidates, sources.InboxCandidate{
			SourceID:   s.id,
			ExternalID: "tick-" + fired.Format(time.RFC3339Nano),
			EntityType: s.entityType,
			Title:      "scheduled tick",
			RawPayload: payload,
			Normalized: payload,
			ObservedAt: fired,
		})
		cursorTime = fired
	}
	if len(candidates) == 0 {
		return nil, cursor, nil
	}
	return candidates, sources.Cursor{Value: cursorTime.Format(time.RFC3339Nano)}, nil
}
