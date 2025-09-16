package models

import "time"

// AccountsCache represents cached accounts list data
type AccountsCache struct {
	Accounts []Account `json:"accounts"`
	CachedAt time.Time `json:"cached_at"`
	TTL      int       `json:"ttl"`
}

// PipelinesCache represents cached pipelines list data
type PipelinesCache struct {
	Pipelines []Pipeline `json:"pipelines"`
	CachedAt  time.Time  `json:"cached_at"`
	TTL       int        `json:"ttl"`
}

// PipelineDetailCache represents cached individual pipeline detail data
type PipelineDetailCache struct {
	Pipeline Pipeline  `json:"pipeline"`
	CachedAt time.Time `json:"cached_at"`
	TTL      int       `json:"ttl"`
}

// PipelineStateCache represents cached individual pipeline state data
type PipelineStateCache struct {
	State    *PipelineState `json:"state"`
	CachedAt time.Time      `json:"cached_at"`
	TTL      int            `json:"ttl"`
}

// PipelineExecutionsCache represents cached pipeline executions data
type PipelineExecutionsCache struct {
	Executions []PipelineExecutionSummary `json:"executions"`
	CachedAt   time.Time                  `json:"cached_at"`
	TTL        int                        `json:"ttl"`
}

// Legacy cache types for backward compatibility
// Deprecated: Use AccountsCache instead
type AccountCache = AccountsCache

// Deprecated: Use PipelinesCache instead
type PipelineCache = PipelinesCache

// Deprecated: Use PipelineExecutionsCache instead
type PipelineExecutionCache = PipelineExecutionsCache

// PipelineDetailsCache represents cached pipeline details data (legacy)
// Deprecated: Use PipelineDetailCache for individual pipelines instead
type PipelineDetailsCache struct {
	Pipelines []Pipeline `json:"pipelines"`
	CachedAt  time.Time  `json:"cached_at"`
	TTL       int        `json:"ttl"`
}

// SimplePipelineCache represents the legacy simplified pipeline cache format
// Deprecated: Use PipelineDetailCache instead
type SimplePipelineCache struct {
	Pipeline SimplePipeline `json:"pipeline"`
	CachedAt time.Time      `json:"cached_at"`
	TTL      int            `json:"ttl"`
}
