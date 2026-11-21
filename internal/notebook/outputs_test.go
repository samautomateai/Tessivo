package notebook

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/samautomateai/tessivo/internal/worktype"
)

func TestValidID(t *testing.T) {
	if !ValidID(NewID()) {
		t.Fatal("NewID is not valid")
	}
	for _, bad := range []string{"", "../x", "20261121T100000-ABCDEF", "20261121T100000-abcdef.json"} {
		if ValidID(bad) {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestGetOutputRejectsBadID(t *testing.T) {
	nb := newNB(t)
	if _, _, err := nb.GetOutput("../outputs/x"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want an invalid-id error, got %v", err)
	}
}

func TestGetOutputMissing(t *testing.T) {
	nb := newNB(t)
	if _, _, err := nb.GetOutput(NewID()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
}

func TestOutputParentRoundTrip(t *testing.T) {
	nb := newNB(t)
	o := Output{
		ID: NewID(), RunID: NewID(), WorkType: worktype.Analyze, ParentID: NewID(), FromThread: "m1",
		Data: []byte(`{}`),
	}
	if err := nb.SaveOutput(o, "# md"); err != nil {
		t.Fatal(err)
	}
	got, md, err := nb.GetOutput(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != o.ParentID || got.FromThread != "m1" || md != "# md" {
		t.Fatalf("round trip: %+v %q", got, md)
	}
}

func TestRunPromptNameRoundTrip(t *testing.T) {
	nb := newNB(t)
	r := Run{ID: NewID(), PromptName: "weekly"}
	if err := nb.SaveRun(r); err != nil {
		t.Fatal(err)
	}
	got, err := nb.LoadRun(r.ID)
	if err != nil || got.PromptName != "weekly" {
		t.Fatalf("run: %+v %v", got, err)
	}
}
