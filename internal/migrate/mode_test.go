package migrate

import "testing"

func TestParseRunMode(t *testing.T) {
	got, err := ParseRunMode("")
	if err != nil || got != RunModeDiff {
		t.Fatalf("empty string: got mode=%q err=%v", got, err)
	}
	for _, in := range []string{"diff", "DIFF", " Diff "} {
		got, err := ParseRunMode(in)
		if err != nil || got != RunModeDiff {
			t.Fatalf("ParseRunMode(%q) = %q, %v; want diff, nil", in, got, err)
		}
	}
	got, err = ParseRunMode("all")
	if err != nil || got != RunModeAll {
		t.Fatalf(`ParseRunMode("all") = %q, %v; want all, nil`, got, err)
	}
	if _, err := ParseRunMode("bogus"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}
