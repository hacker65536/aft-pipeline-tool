package models

import "time"

// PipelineCache represents cached pipeline data
type PipelineCache struct {
	Pipelines []Pipeline `json:"pipelines"`
	CachedAt  time.Time  `json:"cached_at"`
	TTL       int        `json:"ttl"`
}

// PipelineDetailsCache represents cached pipeline details data
type PipelineDetailsCache struct {
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

// SimplePipelineCache represents the legacy simplified pipeline cache format
type SimplePipelineCache struct {
	Pipeline SimplePipeline `json:"pipeline"`
	CachedAt time.Time      `json:"cached_at"`
	TTL      int            `json:"ttl"`
}
