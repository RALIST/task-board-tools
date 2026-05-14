package agent

import (
	"time"
)

// SupportedUsageAgents enumerates which agent CLIs the usage collectors know
// how to read. Add a new entry here (and a parser file alongside) when adding
// support for another agent.
var SupportedUsageAgents = []string{"claude", "codex"}

// UsageWindow is one rate-limit bucket reported by an agent CLI. Agents
// typically expose two windows (e.g. a 5-hour rolling bucket and a weekly
// bucket); both are normalised into the same shape.
//
// All fields are optional. UsedPercent==nil means "the parser found this
// window but didn't see a usable percentage"; an entirely absent window is
// represented by a nil *UsageWindow on the parent Usage.
type UsageWindow struct {
	// UsedPercent is in 0..100. nil when the agent reports an unknown value.
	UsedPercent *float64 `json:"usedPercent,omitempty"`
	// WindowLabel is a short human label ("5h", "weekly"). Empty when unknown.
	WindowLabel string `json:"windowLabel,omitempty"`
	// ResetsAt is the unix time at which this window resets. Zero when unknown.
	ResetsAt time.Time `json:"resetsAt,omitzero"`
}

// Usage is the normalized usage snapshot for one agent CLI, ready to send
// straight to the frontend. The frontend renders only this struct — no
// per-agent parsing happens in Svelte.
type Usage struct {
	// Agent is one of SupportedUsageAgents.
	Agent string `json:"agent"`
	// Available means we have a usable snapshot. When false the frontend renders
	// "unknown" and consults Reason for a tooltip.
	Available bool `json:"available"`
	// Reason explains an unavailable state (agent not installed, no sessions
	// yet, parser failed, etc.). Always empty when Available==true.
	Reason string `json:"reason,omitempty"`
	// Primary is the short-window bucket (typically a 5h rolling window).
	Primary *UsageWindow `json:"primary,omitempty"`
	// Secondary is the long-window bucket (typically a weekly bucket).
	Secondary *UsageWindow `json:"secondary,omitempty"`
	// Plan is the plan label the agent reports (e.g. "max", "prolite"). Empty
	// when the source did not include it.
	Plan string `json:"plan,omitempty"`
	// Source is a short identifier of where the value came from
	// ("codex-session-jsonl", "claude-oauth-usage", "stub"). Useful for
	// debugging and for the frontend tooltip.
	Source string `json:"source,omitempty"`
	// LastUpdated is when this snapshot was produced.
	LastUpdated time.Time `json:"lastUpdated"`
}

// unknownUsage builds an Available:false Usage with the given reason. Used by
// every collector so the "unknown" shape stays consistent.
func unknownUsage(agent, reason string) Usage {
	return Usage{
		Agent:       agent,
		Available:   false,
		Reason:      reason,
		LastUpdated: time.Now().UTC(),
	}
}
