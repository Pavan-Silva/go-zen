package cron

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Scheduler manages periodic jobs using cron expressions.
type Scheduler struct {
	jobs    []*job
	running bool
	stop    chan struct{}
	mu      sync.Mutex
}

type job struct {
	spec     string
	fn       func()
	next     time.Time
	parsed   cronSpec
	disabled bool
}

type cronSpec struct {
	minute  []int
	hour    []int
	day     []int
	month   []int
	weekday []int
}

// New creates a new cron scheduler.
func New() *Scheduler {
	return &Scheduler{
		jobs: make([]*job, 0),
		stop: make(chan struct{}),
	}
}

// AddJob adds a job with a cron expression.
// Cron format: "minute hour day month weekday"
// Use * for any, */n for every n, or comma-separated values.
// Example: "*/5 * * * *" runs every 5 minutes.
func (s *Scheduler) AddJob(spec string, fn func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parsed, err := parseCron(spec)
	if err != nil {
		return fmt.Errorf("cron: invalid spec %q: %w", spec, err)
	}

	j := &job{
		spec:   spec,
		fn:     fn,
		parsed: parsed,
	}
	j.next = nextTime(j.parsed, time.Now())

	s.jobs = append(s.jobs, j)
	sort.Slice(s.jobs, func(i, j int) bool {
		return s.jobs[i].next.Before(s.jobs[j].next)
	})

	return nil
}

// Start begins executing scheduled jobs.
// It runs in a goroutine and returns immediately.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop halts execution of scheduled jobs.
// It waits for any running job to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stop)
	s.mu.Unlock()
}

// run is the main scheduler loop.
func (s *Scheduler) run() {
	for {
		s.mu.Lock()
		if !s.running || len(s.jobs) == 0 {
			s.mu.Unlock()
			return
		}

		next := s.jobs[0].next
		s.mu.Unlock()

		select {
		case <-time.After(time.Until(next)):
			s.mu.Lock()
			if !s.running {
				s.mu.Unlock()
				return
			}

			// Execute due jobs
			now := time.Now()
			for len(s.jobs) > 0 && !s.jobs[0].next.After(now) {
				j := s.jobs[0]
				if !j.disabled {
					go j.fn() // Run in goroutine to avoid blocking
				}
				j.next = nextTime(j.parsed, now)
				s.jobs = s.jobs[1:] // Remove from front

				// Re-insert with new next time
				s.jobs = append(s.jobs, j)
			}

			// Re-sort jobs by next execution time
			sort.Slice(s.jobs, func(i, j int) bool {
				return s.jobs[i].next.Before(s.jobs[j].next)
			})
			s.mu.Unlock()

		case <-s.stop:
			return
		}
	}
}

// parseCron parses a cron expression into a cronSpec.
// Supports: * */n a,b,c ranges
func parseCron(spec string) (cronSpec, error) {
	parts := strings.Fields(spec)
	if len(parts) != 5 {
		return cronSpec{}, errors.New("must have 5 fields: minute hour day month weekday")
	}

	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return cronSpec{}, fmt.Errorf("minute: %w", err)
	}

	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return cronSpec{}, fmt.Errorf("hour: %w", err)
	}

	day, err := parseField(parts[2], 1, 31)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day: %w", err)
	}

	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return cronSpec{}, fmt.Errorf("month: %w", err)
	}

	weekday, err := parseField(parts[4], 0, 6)
	if err != nil {
		return cronSpec{}, fmt.Errorf("weekday: %w", err)
	}

	return cronSpec{
		minute:  minute,
		hour:    hour,
		day:     day,
		month:   month,
		weekday: weekday,
	}, nil
}

// parseField parses a single cron field.
func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		values := make([]int, 0, max-min+1)
		for i := min; i <= max; i++ {
			values = append(values, i)
		}
		return values, nil
	}

	var values []int
	parts := strings.SplitSeq(field, ",")
	for part := range parts {
		step := 1
		if strings.Contains(part, "/") {
			subparts := strings.Split(part, "/")
			if len(subparts) != 2 {
				return nil, errors.New("invalid step syntax")
			}
			part = subparts[0]
			_, err := fmt.Sscanf(subparts[1], "%d", &step)
			if err != nil || step < 1 {
				return nil, errors.New("invalid step value")
			}
		}

		if part == "*" {
			for i := min; i <= max; i += step {
				values = append(values, i)
			}
		} else if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, errors.New("invalid range")
			}
			start, err := parseInt(rangeParts[0], min, max)
			if err != nil {
				return nil, err
			}
			end, err := parseInt(rangeParts[1], min, max)
			if err != nil {
				return nil, err
			}
			for i := start; i <= end; i += step {
				values = append(values, i)
			}
		} else {
			val, err := parseInt(part, min, max)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
	}

	// Sort and deduplicate
	sort.Ints(values)
	result := make([]int, 0, len(values))
	for i, v := range values {
		if i == 0 || v != values[i-1] {
			result = append(result, v)
		}
	}
	return result, nil
}

func parseInt(s string, min, max int) (int, error) {
	if s == "*" {
		return 0, errors.New("unexpected *")
	}
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return 0, err
	}
	if val < min || val > max {
		return 0, fmt.Errorf("value %d out of range [%d,%d]", val, min, max)
	}
	return val, nil
}

// nextTime calculates the next execution time for a cron spec.
func nextTime(spec cronSpec, from time.Time) time.Time {
	// Start from the next minute
	t := from.Add(time.Minute - time.Duration(from.Second())*time.Second - time.Duration(from.Nanosecond()))

	for {
		if contains(spec.month, int(t.Month())) &&
			contains(spec.day, t.Day()) &&
			contains(spec.weekday, int(t.Weekday())) &&
			contains(spec.hour, t.Hour()) &&
			contains(spec.minute, t.Minute()) {
			return t
		}
		t = t.Add(time.Minute)
	}
}

func contains(slice []int, val int) bool {
	return slices.Contains(slice, val)
}