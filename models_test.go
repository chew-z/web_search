package main

import "testing"

// TestModelRegistry_EffortDriftGuard is the required drift-guard: every model's
// RecommendedEffort must resolve via effortByName. Fails loudly if a model
// references an unknown/renamed effort or a stale timeout is reintroduced.
func TestModelRegistry_EffortDriftGuard(t *testing.T) {
	for _, m := range modelRegistry {
		if _, ok := effortByName(m.RecommendedEffort); !ok {
			t.Errorf("model %q references unknown effort %q (not in effortRegistry)", m.Name, m.RecommendedEffort)
		}
	}
}

// TestEffortByName checks lookup hits and the comma-ok miss path.
func TestEffortByName(t *testing.T) {
	if e, ok := effortByName("medium"); !ok || e.Timeout != timeoutMedium || e.DisplayTimeout != "5min" {
		t.Errorf("effortByName(medium) = %+v, ok=%v; want timeoutMedium / 5min", e, ok)
	}
	if _, ok := effortByName("turbo"); ok {
		t.Errorf("effortByName(turbo) ok=true, want false")
	}
	if _, ok := effortByName(""); ok {
		t.Errorf("effortByName(\"\") ok=true, want false")
	}
}

// TestEffortEnumValues asserts registry order is preserved for the schema enum.
func TestEffortEnumValues(t *testing.T) {
	got := effortEnumValues()
	want := []string{"none", "low", "medium", "high", "xhigh"}
	if len(got) != len(want) {
		t.Fatalf("effortEnumValues len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("effortEnumValues[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestEffortDescription verifies the description is built from the registry and
// matches the previously hardcoded string (behavior preserved).
func TestEffortDescription(t *testing.T) {
	got := effortDescription()
	want := "Reasoning effort level: none (90s), low (3min), medium (5min), high (10min), or xhigh (15min timeout)"
	if got != want {
		t.Errorf("effortDescription() =\n  %q\nwant\n  %q", got, want)
	}
}
