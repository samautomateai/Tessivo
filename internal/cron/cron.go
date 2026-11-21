// Package cron parses standard five-field cron expressions and computes the
// next matching time. Supported: numbers, names (JAN-DEC, SUN-SAT), "*",
// ranges "a-b", steps "*/n" and "a-b/n", lists "a,b", macros (@yearly,
// @monthly, @weekly, @daily, @hourly) and a "CRON_TZ=Area/City " prefix.
// When both day-of-month and day-of-week are restricted, either may match
// (classic cron semantics). Times are computed in the schedule's location,
// so daylight-saving changes behave like wall-clock cron.
package cron

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxSpecLen bounds expression length.
const MaxSpecLen = 256

// Schedule is a parsed expression.
type Schedule struct {
	minute, hour, dom, month, dow uint64
	domStar, dowStar              bool
	loc                           *time.Location
	spec                          string
}

type field struct {
	name     string
	min, max int
	names    map[string]int
}

var (
	minuteF = field{name: "minute", min: 0, max: 59}
	hourF   = field{name: "hour", min: 0, max: 23}
	domF    = field{name: "day of month", min: 1, max: 31}
	monthF  = field{name: "month", min: 1, max: 12, names: map[string]int{
		"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
		"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	}}
	dowF = field{name: "day of week", min: 0, max: 7, names: map[string]int{
		"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
	}}
)

var macros = map[string]string{
	"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
	"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
}

// Parse parses spec. Times are evaluated in loc unless spec has CRON_TZ.
func Parse(spec string, loc *time.Location) (*Schedule, error) {
	if len(spec) > MaxSpecLen {
		return nil, fmt.Errorf("cron: expression longer than %d characters", MaxSpecLen)
	}
	if loc == nil {
		loc = time.Local
	}
	s := &Schedule{spec: strings.TrimSpace(spec), loc: loc}
	expr := s.spec
	if rest, ok := strings.CutPrefix(expr, "CRON_TZ="); ok {
		tz, tail, _ := strings.Cut(rest, " ")
		l, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("cron: unknown time zone %q", tz)
		}
		s.loc, expr = l, strings.TrimSpace(tail)
	}
	if m, ok := macros[strings.ToLower(expr)]; ok {
		expr = m
	}
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron: want 5 fields (minute hour day month weekday), got %d", len(parts))
	}
	var err error
	fields := []struct {
		dst *uint64
		f   field
	}{{&s.minute, minuteF}, {&s.hour, hourF}, {&s.dom, domF}, {&s.month, monthF}, {&s.dow, dowF}}
	for i, fd := range fields {
		if *fd.dst, err = parseField(parts[i], fd.f); err != nil {
			return nil, err
		}
	}
	if s.dow&(1<<7) != 0 { // 7 is also Sunday
		s.dow |= 1
	}
	s.domStar, s.dowStar = parts[2] == "*" || parts[2] == "?", parts[4] == "*" || parts[4] == "?"
	return s, nil
}

func parseField(text string, f field) (uint64, error) {
	var set uint64
	for _, item := range strings.Split(text, ",") {
		bitsFor, err := parseItem(item, f)
		if err != nil {
			return 0, err
		}
		set |= bitsFor
	}
	return set, nil
}

func parseItem(item string, f field) (uint64, error) {
	rangePart, stepPart, hasStep := strings.Cut(item, "/")
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepPart)
		if err != nil || n < 1 || n > f.max {
			return 0, fmt.Errorf("cron: bad step %q in %s", stepPart, f.name)
		}
		step = n
	}
	lo, hi := f.min, f.max
	switch rangePart {
	case "*", "?":
	default:
		a, b, isRange := strings.Cut(rangePart, "-")
		var err error
		if lo, err = value(a, f); err != nil {
			return 0, err
		}
		hi = lo
		if isRange {
			if hi, err = value(b, f); err != nil {
				return 0, err
			}
		} else if hasStep {
			hi = f.max
		}
		if lo > hi {
			return 0, fmt.Errorf("cron: range %q is backwards in %s", rangePart, f.name)
		}
	}
	var set uint64
	for v := lo; v <= hi; v += step {
		set |= 1 << v // v is within the field's range (0..59), so the shift is valid
	}
	return set, nil
}

func value(s string, f field) (int, error) {
	if n, ok := f.names[strings.ToUpper(s)]; ok {
		return n, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < f.min || n > f.max {
		return 0, fmt.Errorf("cron: %q is not a valid %s (%d-%d)", s, f.name, f.min, f.max)
	}
	return n, nil
}

// String returns the original expression.
func (s *Schedule) String() string { return s.spec }

// Location returns the time zone the schedule runs in.
func (s *Schedule) Location() *time.Location { return s.loc }

// ErrNoMatch means no time matches within the search horizon (e.g. 30 Feb).
var ErrNoMatch = errors.New("cron: no matching time within 5 years")

func has(set uint64, v int) bool { return v >= 0 && v < 64 && set&(1<<v) != 0 }

func (s *Schedule) dayMatches(t time.Time) bool {
	dom, dow := has(s.dom, t.Day()), has(s.dow, int(t.Weekday()))
	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return dow
	case s.dowStar:
		return dom
	default:
		return dom || dow
	}
}

// wall returns t's wall-clock reading, for comparing local times across DST.
func wall(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

// Next returns the first matching time strictly after after.
func (s *Schedule) Next(after time.Time) (time.Time, error) {
	t := after.In(s.loc)
	floor := wall(t)
	t = t.Add(-time.Duration(t.Second())*time.Second - time.Duration(t.Nanosecond()) + time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		switch {
		case !has(s.month, int(t.Month())):
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, s.loc)
		case !s.dayMatches(t):
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, s.loc)
		case !has(s.hour, t.Hour()):
			// Absolute step to the next wall-clock hour; works for :30/:45 offsets
			// and steps through DST transitions without re-entering an hour.
			t = t.Add(time.Duration(60-t.Minute()) * time.Minute)
		case !has(s.minute, t.Minute()):
			t = t.Add(time.Minute)
		case !wall(t).After(floor):
			// DST fall-back repeats wall-clock times; fire each one only once.
			t = t.Add(time.Minute)
		default:
			return t, nil
		}
	}
	return time.Time{}, ErrNoMatch
}
