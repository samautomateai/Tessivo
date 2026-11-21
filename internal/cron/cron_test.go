package cron

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, spec string, loc *time.Location) *Schedule {
	t.Helper()
	s, err := Parse(spec, loc)
	if err != nil {
		t.Fatalf("Parse(%q): %v", spec, err)
	}
	return s
}

func at(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNext(t *testing.T) {
	cases := []struct{ spec, from, want string }{
		{"0 8 * * 1", "2026-11-20 23:29", "2026-11-23 08:00"},            // next Monday 08:00
		{"*/15 * * * *", "2026-11-20 10:07", "2026-11-20 10:15"},         // step
		{"0 9-17/4 * * MON-FRI", "2026-11-20 17:00", "2026-11-23 09:00"}, // range step, names
		{"30 2 1 * *", "2026-11-20 00:00", "2026-12-01 02:30"},           // monthly
		{"0 0 29 2 *", "2026-03-01 00:00", "2028-02-29 00:00"},           // leap day
		{"@daily", "2026-11-20 00:00", "2026-11-21 00:00"},
		{"@hourly", "2026-11-20 10:59", "2026-11-20 11:00"},
		{"0 0 1,15 * 0", "2026-11-20 00:00", "2026-11-22 00:00"}, // dom OR dow: Sunday 22nd
		{"0 0 * * 7", "2026-11-20 00:00", "2026-11-22 00:00"},    // 7 is Sunday
		{"5 4 * JAN,DEC *", "2026-11-20 00:00", "2026-12-01 04:05"},
		{"0 12 * * ?", "2026-11-20 12:00", "2026-11-21 12:00"}, // strictly after
		{"1-3 * * * *", "2026-11-20 10:02", "2026-11-20 10:03"},
	}
	for _, c := range cases {
		got, err := mustParse(t, c.spec, time.UTC).Next(at(c.from))
		if err != nil || !got.Equal(at(c.want)) {
			t.Errorf("%s from %s: got %v (%v), want %s", c.spec, c.from, got, err, c.want)
		}
	}
}

func TestTimeZonesAndDST(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Skip("tzdata not available")
	}
	s := mustParse(t, "CRON_TZ=Europe/London 30 1 * * *", time.UTC)
	if s.Location() != london && s.Location().String() != "Europe/London" {
		t.Fatalf("location %v", s.Location())
	}
	// Spring forward 2026-03-29: 01:00-02:00 does not exist; 01:30 is skipped that day.
	got, _ := s.Next(time.Date(2026, 3, 28, 12, 0, 0, 0, london))
	if want := time.Date(2026, 3, 30, 1, 30, 0, 0, london); !got.Equal(want) {
		t.Fatalf("spring forward: got %v want %v", got, want)
	}
	// Fall back 2026-10-25: 01:30 happens twice; we fire once, then move on.
	first, _ := s.Next(time.Date(2026, 10, 24, 12, 0, 0, 0, london))
	second, _ := s.Next(first)
	if first.Day() != 25 || second.Day() != 26 || second.Hour() != 1 || second.Minute() != 30 {
		t.Fatalf("fall back: %v then %v", first, second)
	}
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err == nil {
		h := mustParse(t, "0 * * * *", kolkata)
		got, _ := h.Next(time.Date(2026, 1, 1, 10, 10, 0, 0, kolkata))
		if got.Minute() != 0 || got.Hour() != 11 {
			t.Fatalf("half-hour offset: %v", got)
		}
	}
	if _, err := Parse("CRON_TZ=Mars/Olympus 0 0 * * *", nil); err == nil {
		t.Fatal("want unknown zone")
	}
	if mustParse(t, "0 0 * * *", nil).Location() != time.Local {
		t.Fatal("nil location means local")
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "* * * * 8",
		"5-1 * * * *", "*/0 * * * *", "*/x * * * *", "a * * * *", "* * * FOO *", "1,,2 * * * *",
		strings.Repeat("1", MaxSpecLen+1),
	}
	for _, spec := range bad {
		if _, err := Parse(spec, time.UTC); err == nil {
			t.Errorf("Parse(%q): want error", spec)
		}
	}
}

func TestNoMatch(t *testing.T) {
	s := mustParse(t, "0 0 30 2 *", time.UTC)
	if _, err := s.Next(at("2026-01-01 00:00")); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("want ErrNoMatch, got %v", err)
	}
	if s.String() != "0 0 30 2 *" {
		t.Fatal("String")
	}
}

func FuzzParseNext(f *testing.F) {
	for _, s := range []string{"0 8 * * 1", "*/7 1-5 */3 JAN-MAR SUN", "@weekly", "CRON_TZ=UTC 1 2 3 4 5"} {
		f.Add(s)
	}
	from := at("2026-11-20 12:00")
	f.Fuzz(func(t *testing.T, spec string) {
		s, err := Parse(spec, time.UTC)
		if err != nil {
			return
		}
		got, err := s.Next(from)
		if err != nil {
			return
		}
		if !got.After(from) || got.Second() != 0 {
			t.Fatalf("%q: Next(%v) = %v", spec, from, got)
		}
		if !has(s.minute, got.Minute()) || !has(s.hour, got.Hour()) || !has(s.month, int(got.Month())) ||
			!s.dayMatches(got) {
			t.Fatalf("%q: %v does not match", spec, got)
		}
	})
}

func BenchmarkNext(b *testing.B) {
	s, _ := Parse("0 9 * * MON-FRI", time.UTC)
	from := at("2026-11-20 12:00")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := s.Next(from); err != nil {
			b.Fatal(err)
		}
	}
}
