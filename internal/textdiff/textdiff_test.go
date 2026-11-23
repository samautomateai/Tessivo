package textdiff

import (
	"strings"
	"testing"
)

func rebuild(lines []Line) (a, b string) {
	var as, bs []string
	for _, l := range lines {
		if l.Op != Add {
			as = append(as, l.Text)
		}
		if l.Op != Remove {
			bs = append(bs, l.Text)
		}
	}
	return strings.Join(as, "\n"), strings.Join(bs, "\n")
}

func TestLines(t *testing.T) {
	a := "# Report\nScore 7\nShip in March\nOwner: Ann"
	b := "# Report\nScore 8\nShip in March\nOwner: Ann\nRisk: low"
	got := Lines(a, b)
	want := []Line{
		{Same, "# Report"},
		{Remove, "Score 7"},
		{Add, "Score 8"},
		{Same, "Ship in March"},
		{Same, "Owner: Ann"},
		{Add, "Risk: low"},
	}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if s := Summary(got); s.Added != 2 || s.Removed != 1 {
		t.Fatalf("summary %+v", s)
	}
}

func TestLargeChangesStayBoundedAndCorrect(t *testing.T) {
	a := strings.Repeat("old line\n", 3000) + "end"
	b := strings.Repeat("new line\n", 3000) + "end"
	got := Lines(a, b)
	if ra, rb := rebuild(got); ra != a || rb != b {
		t.Fatal("a large diff does not rebuild both texts")
	}
}

func FuzzLines(f *testing.F) {
	f.Add("a\nb\nc", "a\nc\nd")
	f.Add("", "x")
	f.Add("same", "same")
	f.Fuzz(func(t *testing.T, a, b string) {
		got := Lines(a, b)
		if ra, rb := rebuild(got); ra != a || rb != b {
			t.Fatalf("rebuild: %q %q from %q %q", ra, rb, a, b)
		}
	})
}
