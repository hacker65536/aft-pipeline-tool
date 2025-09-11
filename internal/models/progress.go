package models

import (
	"fmt"
	"sync"
	"time"
)

// ProgressInfo represents progress information for pipeline details retrieval
type ProgressInfo struct {
	Total       int           `json:"total"`
	Current     int           `json:"current"`
	FromCache   int           `json:"from_cache"`
	FromAPI     int           `json:"from_api"`
	StartTime   time.Time     `json:"start_time"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	mutex       sync.RWMutex
}

// NewProgressInfo creates a new progress info instance
func NewProgressInfo(total int) *ProgressInfo {
	return &ProgressInfo{
		Total:     total,
		Current:   0,
		FromCache: 0,
		FromAPI:   0,
		StartTime: time.Now(),
	}
}

// IncrementCache increments the cache counter and current progress
func (p *ProgressInfo) IncrementCache() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.FromCache++
	p.Current++
	p.ElapsedTime = time.Since(p.StartTime)
}

// IncrementAPI increments the API counter and current progress
func (p *ProgressInfo) IncrementAPI() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.FromAPI++
	p.Current++
	p.ElapsedTime = time.Since(p.StartTime)
}

// GetProgress returns the current progress information (thread-safe)
func (p *ProgressInfo) GetProgress() (int, int, int, int, time.Duration) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.Total, p.Current, p.FromCache, p.FromAPI, p.ElapsedTime
}

// GetProgressString returns a formatted progress string
func (p *ProgressInfo) GetProgressString() string {
	total, current, fromCache, fromAPI, elapsed := p.GetProgress()

	percentage := 0.0
	if total > 0 {
		percentage = float64(current) / float64(total) * 100
	}

	return fmt.Sprintf("Progress: %d/%d (%.1f%%) - Cache: %d, API: %d - Elapsed: %v",
		current, total, percentage, fromCache, fromAPI, elapsed.Truncate(time.Millisecond))
}

// IsComplete returns true if all pipelines have been processed
func (p *ProgressInfo) IsComplete() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.Current >= p.Total
}

// GetCompletionSummary returns a summary string when processing is complete
func (p *ProgressInfo) GetCompletionSummary() string {
	_, current, fromCache, fromAPI, elapsed := p.GetProgress()

	return fmt.Sprintf("Completed: %d pipelines processed in %v (Cache: %d, API: %d)",
		current, elapsed.Truncate(time.Millisecond), fromCache, fromAPI)
}
