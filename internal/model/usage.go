// Package model holds the dependency-free data types that represent a parsed
// Claude Code session. The JSON renderer marshals these directly, so the JSON
// tags here define the json output shape.
package model

// Usage is the token accounting for a single assistant API response.
type Usage struct {
	InputTokens         int    `json:"input_tokens"`
	OutputTokens        int    `json:"output_tokens"`
	CacheReadTokens     int    `json:"cache_read_input_tokens"`
	CacheCreationTokens int    `json:"cache_creation_input_tokens"`
	ServiceTier         string `json:"service_tier,omitempty"`
}

// ContextTokens is the total context size implied by a response: fresh input
// plus everything read from or written to the prompt cache.
func (u Usage) ContextTokens() int {
	return u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
}
