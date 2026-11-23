package worktype

import "testing"

func TestLibraryCoversEveryWorkTypeWithUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	types := map[string]int{}
	for _, p := range Library {
		if seen[p.ID] || p.ID == "" || p.Name == "" || len(p.Text) < 40 {
			t.Fatalf("bad entry %+v", p)
		}
		seen[p.ID] = true
		if _, err := Get(p.WorkType); err != nil {
			t.Fatalf("%s: %v", p.ID, err)
		}
		types[p.WorkType]++
	}
	for _, w := range []string{"analyze", "summarize", "document", "report"} {
		if types[w] < 2 {
			t.Fatalf("%s has %d library prompts; want at least 2", w, types[w])
		}
	}
	for _, id := range []string{"action-items", "draft-doc", "triage-issue"} {
		if !seen[id] {
			t.Fatalf("the spec's starter %s is missing", id)
		}
	}
	if got := LibraryFor("report"); len(got) != types["report"] || got[0].WorkType != "report" {
		t.Fatalf("filter: %+v", got)
	}
	if len(LibraryFor("")) != len(Library) || len(LibraryFor("nope")) != 0 {
		t.Fatal("empty id lists all; an unknown one lists none")
	}
}
