package cache

import (
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Account operations
	GetAccounts() (*models.AccountsCache, error)
	SetAccounts(accounts []models.Account, ttl int) error

	// Pipeline operations
	GetPipelines() (*models.PipelinesCache, error)
	SetPipelines(pipelines []models.Pipeline, ttl int) error

	// Pipeline detail operations
	GetPipelineDetail(pipelineName string) (*models.PipelineDetailCache, error)
	SetPipelineDetail(pipeline models.Pipeline, ttl int) error

	// Pipeline state operations
	GetPipelineState(pipelineName string) (*models.PipelineStateCache, error)
	SetPipelineState(pipelineName string, state *models.PipelineState, ttl int) error

	// Pipeline execution operations
	GetPipelineExecutions(pipelineName string) (*models.PipelineExecutionsCache, error)
	SetPipelineExecutions(pipelineName string, executions []models.PipelineExecutionSummary, ttl int) error

	// Cache management
	ClearCache() error
	DeletePipelineCache(pipelineName string) error
}

// CacheItem represents a generic cache item with TTL
type CacheItem struct {
	Data     interface{} `json:"data"`
	CachedAt time.Time   `json:"cached_at"`
	TTL      int         `json:"ttl"`
}

// IsExpired checks if the cache item has expired
func (c *CacheItem) IsExpired() bool {
	return time.Since(c.CachedAt).Seconds() > float64(c.TTL)
}

// NewCacheItem creates a new cache item
func NewCacheItem(data interface{}, ttl int) *CacheItem {
	return &CacheItem{
		Data:     data,
		CachedAt: time.Now(),
		TTL:      ttl,
	}
}
