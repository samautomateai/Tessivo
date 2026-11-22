package format

import (
	"errors"
	"testing"
)

type doc struct{ Steps []int }

func TestUpgradeRunsEachStepInOrder(t *testing.T) {
	steps := []func(*doc) error{
		func(d *doc) error { d.Steps = append(d.Steps, 2); return nil },
		func(d *doc) error { d.Steps = append(d.Steps, 3); return nil },
	}
	for _, tc := range []struct{ from, want int }{{0, 3}, {1, 3}, {2, 3}, {3, 3}} {
		d, v := &doc{}, tc.from
		if err := Upgrade("notebook", &v, 3, steps, d); err != nil || v != tc.want {
			t.Fatalf("from %d: %v, version %d", tc.from, err, v)
		}
		if tc.from <= 1 && len(d.Steps) != 2 || tc.from == 2 && len(d.Steps) != 1 || tc.from == 3 && len(d.Steps) != 0 {
			t.Fatalf("from %d ran %v", tc.from, d.Steps)
		}
	}
}

func TestUpgradeRefusesNewerAndStopsOnError(t *testing.T) {
	v := 4
	if err := Upgrade("notebook", &v, 3, nil, &doc{}); !errors.Is(err, ErrNewer) {
		t.Fatalf("newer: %v", err)
	}
	v = 1
	boom := errors.New("boom")
	fail := []func(*doc) error{func(*doc) error { return boom }}
	if err := Upgrade("notebook", &v, 2, fail, &doc{}); !errors.Is(err, boom) || v != 1 {
		t.Fatalf("failed step: %v, version %d", err, v)
	}
}
