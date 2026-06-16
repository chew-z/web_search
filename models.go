package main

import (
	"strings"
	"time"
)

// effortInfo is the single source of truth for a reasoning-effort level: its
// canonical name, the human-readable timeout shown in schemas/descriptions, and
// the actual duration applied to requests. Nothing else may re-store a timeout.
type effortInfo struct {
	Name           string
	DisplayTimeout string
	Timeout        time.Duration
}

// effortRegistry is the ONE place effort name + timeout live. Adding or removing
// a level here propagates to validation, the tool's JSON-schema enum, the tool
// description, and the models resource simultaneously.
var effortRegistry = []effortInfo{
	{"none", "90s", timeoutNone},
	{"low", "3min", timeoutLow},
	{"medium", "5min", timeoutMedium},
	{"high", "10min", timeoutHigh},
	{"xhigh", "15min", timeoutXHigh},
}

// verbosityLevels is the single source of truth for valid verbosity values.
var verbosityLevels = []string{"low", "medium", "high"}

// effortByName looks up an effort level by its canonical name. The bool reports
// whether the name is known (mirrors map-style comma-ok lookups).
func effortByName(name string) (effortInfo, bool) {
	for _, e := range effortRegistry {
		if e.Name == name {
			return e, true
		}
	}
	return effortInfo{}, false
}

// effortEnumValues returns the effort names in registry order, for mcp.Enum.
func effortEnumValues() []string {
	out := make([]string, len(effortRegistry))
	for i, e := range effortRegistry {
		out[i] = e.Name
	}
	return out
}

// effortDescription builds the reasoning_effort field description from the
// registry, e.g. "Reasoning effort level: none (90s), low (3min), medium
// (5min), high (10min), or xhigh (15min timeout)".
func effortDescription() string {
	parts := make([]string, len(effortRegistry))
	for i, e := range effortRegistry {
		label := e.DisplayTimeout
		if i == len(effortRegistry)-1 {
			label += " timeout"
		}
		parts[i] = e.Name + " (" + label + ")"
	}
	if len(parts) == 0 {
		return "Reasoning effort level"
	}
	joined := parts[len(parts)-1]
	if len(parts) > 1 {
		joined = strings.Join(parts[:len(parts)-1], ", ") + ", or " + joined
	}
	return "Reasoning effort level: " + joined
}

// modelInfo describes a model. RecommendedEffort is a KEY into effortRegistry —
// never a duplicated timeout. The display timeout is derived via effortByName.
type modelInfo struct {
	Name              string
	Description       string
	RecommendedEffort string // key into effortRegistry
}

// modelRegistry is the single source of truth for the model list surfaced by the
// models resource.
var modelRegistry = []modelInfo{
	{modelNano, "Simple facts, definitions, quick lookups, basic summaries", "none"},
	{modelMini, "Well-defined research tasks, comparisons, specific topics with clear scope", "medium"},
	{modelFull, "Complex analysis, coding questions, multi-faceted problems, reasoning tasks", "high"},
}
