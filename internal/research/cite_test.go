package research

import (
	"testing"
)

func TestParseCitations(t *testing.T) {
	got := ParseCitations("A [1] B [2:auth/session.go] C [9] D [2:../x] E [2:/etc/passwd] F [1]", items())
	if len(got) != 3 {
		t.Fatalf("citations: %+v", got)
	}
	if got[0].N != 1 || got[0].ID != "s1" || got[0].Path != "" {
		t.Errorf("first: %+v", got[0])
	}
	if got[1].N != 2 || got[1].Path != "auth/session.go" || got[1].Label != "payments-service" {
		t.Errorf("second: %+v", got[1])
	}
	if got[2].N != 2 || got[2].Path != "" {
		t.Errorf("bad paths must cite the source only: %+v", got[2])
	}
	if c := ParseCitations("see [3]", items()); len(c) != 1 || c[0].Kind != ItemNote {
		t.Fatalf("note: %+v", c)
	}
}

func TestRelabel(t *testing.T) {
	cites := ParseCitations("see [2:auth/session.go] and [1]", items())
	got := Relabel("see [2:auth/session.go] and [1] and [7]", cites)
	want := "see [payments-service: auth/session.go] and [minutes.md] and [7]"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func FuzzParseCitations(f *testing.F) {
	f.Add("[1] [2:auth/session.go] [0] [99999999999999999999]")
	f.Add("[2:..\\..\\x] [2:a/../../b] [[1]]")
	f.Fuzz(func(t *testing.T, text string) {
		its := items()
		for _, c := range ParseCitations(text, its) {
			if c.N < 1 || c.N > len(its) {
				t.Fatalf("out of range: %+v", c)
			}
			if c.Path != "" && c.Path != "auth/session.go" {
				t.Fatalf("unlisted path: %+v", c)
			}
		}
	})
}
