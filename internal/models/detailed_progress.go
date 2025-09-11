package models

import (
	"fmt"
	"sync"
	"time"
)

// DetailedProgressInfo represents detailed progress information for the entire list operation
type DetailedProgressInfo struct {
	// Account retrieval progress
	AccountsTotal     int       `json:"accounts_total"`
	AccountsCompleted bool      `json:"accounts_completed"`
	AccountsFromCache bool      `json:"accounts_from_cache"`
	AccountsStartTime time.Time `json:"accounts_start_time"`
	AccountsEndTime   time.Time `json:"accounts_end_time"`

	// Pipeline list retrieval progress
	PipelinesTotal     int       `json:"pipelines_total"`
	PipelinesCompleted bool      `json:"pipelines_completed"`
	PipelinesFromCache bool      `json:"pipelines_from_cache"`
	PipelinesStartTime time.Time `json:"pipelines_start_time"`
	PipelinesEndTime   time.Time `json:"pipelines_end_time"`

	// Pipeline details retrieval progress
	DetailsTotal     int       `json:"details_total"`
	DetailsCurrent   int       `json:"details_current"`
	DetailsFromCache int       `json:"details_from_cache"`
	DetailsFromAPI   int       `json:"details_from_api"`
	DetailsStartTime time.Time `json:"details_start_time"`
	DetailsEndTime   time.Time `json:"details_end_time"`

	// Pipeline state retrieval progress (only when --with-state is used)
	StateTotal     int       `json:"state_total"`
	StateCurrent   int       `json:"state_current"`
	StateFromCache int       `json:"state_from_cache"`
	StateFromAPI   int       `json:"state_from_api"`
	StateStartTime time.Time `json:"state_start_time"`
	StateEndTime   time.Time `json:"state_end_time"`

	// Overall progress
	TotalStartTime time.Time `json:"total_start_time"`
	TotalEndTime   time.Time `json:"total_end_time"`

	mutex sync.RWMutex
}

// NewDetailedProgressInfo creates a new detailed progress info instance
func NewDetailedProgressInfo() *DetailedProgressInfo {
	return &DetailedProgressInfo{
		TotalStartTime: time.Now(),
	}
}

// StartAccountsRetrieval starts the accounts retrieval phase
func (p *DetailedProgressInfo) StartAccountsRetrieval() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.AccountsStartTime = time.Now()
	fmt.Printf("🔍 Step 1/4: Retrieving account information...\n")
}

// CompleteAccountsRetrieval completes the accounts retrieval phase
func (p *DetailedProgressInfo) CompleteAccountsRetrieval(total int, fromCache bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.AccountsTotal = total
	p.AccountsCompleted = true
	p.AccountsFromCache = fromCache
	p.AccountsEndTime = time.Now()

	elapsed := p.AccountsEndTime.Sub(p.AccountsStartTime)
	source := "API"
	if fromCache {
		source = "Cache"
	}
	fmt.Printf("✅ Step 1/4: Retrieved %d accounts from %s (%.2fs)\n",
		total, source, elapsed.Seconds())
}

// StartPipelinesRetrieval starts the pipelines retrieval phase
func (p *DetailedProgressInfo) StartPipelinesRetrieval() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.PipelinesStartTime = time.Now()
	fmt.Printf("🔍 Step 2/4: Retrieving pipeline list...\n")
}

// CompletePipelinesRetrieval completes the pipelines retrieval phase
func (p *DetailedProgressInfo) CompletePipelinesRetrieval(total int, fromCache bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.PipelinesTotal = total
	p.PipelinesCompleted = true
	p.PipelinesFromCache = fromCache
	p.PipelinesEndTime = time.Now()

	elapsed := p.PipelinesEndTime.Sub(p.PipelinesStartTime)
	source := "API"
	if fromCache {
		source = "Cache"
	}
	fmt.Printf("✅ Step 2/4: Retrieved %d pipelines from %s (%.2fs)\n",
		total, source, elapsed.Seconds())
}

// StartDetailsRetrieval starts the pipeline details retrieval phase
func (p *DetailedProgressInfo) StartDetailsRetrieval(total int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.DetailsTotal = total
	p.DetailsStartTime = time.Now()
	fmt.Printf("🔍 Step 3/4: Retrieving pipeline details for %d pipelines...\n", total)
}

// IncrementDetailsCache increments the details cache counter
func (p *DetailedProgressInfo) IncrementDetailsCache() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.DetailsFromCache++
	p.DetailsCurrent++
}

// IncrementDetailsAPI increments the details API counter
func (p *DetailedProgressInfo) IncrementDetailsAPI() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.DetailsFromAPI++
	p.DetailsCurrent++
}

// GetDetailsProgressString returns a formatted progress string for details
func (p *DetailedProgressInfo) GetDetailsProgressString() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	percentage := 0.0
	if p.DetailsTotal > 0 {
		percentage = float64(p.DetailsCurrent) / float64(p.DetailsTotal) * 100
	}

	elapsed := time.Since(p.DetailsStartTime)
	return fmt.Sprintf("   Progress: %d/%d (%.1f%%) - Cache: %d, API: %d - Elapsed: %.1fs",
		p.DetailsCurrent, p.DetailsTotal, percentage, p.DetailsFromCache, p.DetailsFromAPI, elapsed.Seconds())
}

// CompleteDetailsRetrieval completes the pipeline details retrieval phase
func (p *DetailedProgressInfo) CompleteDetailsRetrieval() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.DetailsEndTime = time.Now()

	elapsed := p.DetailsEndTime.Sub(p.DetailsStartTime)
	fmt.Printf("✅ Step 3/4: Retrieved %d pipeline details (Cache: %d, API: %d) (%.2fs)\n",
		p.DetailsCurrent, p.DetailsFromCache, p.DetailsFromAPI, elapsed.Seconds())
}

// StartStateRetrieval starts the pipeline state retrieval phase
func (p *DetailedProgressInfo) StartStateRetrieval(total int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.StateTotal = total
	p.StateStartTime = time.Now()
	fmt.Printf("🔍 Step 4/4: Retrieving pipeline state for %d pipelines...\n", total)
}

// IncrementStateCache increments the state cache counter
func (p *DetailedProgressInfo) IncrementStateCache() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.StateFromCache++
	p.StateCurrent++
}

// IncrementStateAPI increments the state API counter
func (p *DetailedProgressInfo) IncrementStateAPI() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.StateFromAPI++
	p.StateCurrent++
}

// GetStateProgressString returns a formatted progress string for state
func (p *DetailedProgressInfo) GetStateProgressString() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	percentage := 0.0
	if p.StateTotal > 0 {
		percentage = float64(p.StateCurrent) / float64(p.StateTotal) * 100
	}

	elapsed := time.Since(p.StateStartTime)
	return fmt.Sprintf("   Progress: %d/%d (%.1f%%) - Cache: %d, API: %d - Elapsed: %.1fs",
		p.StateCurrent, p.StateTotal, percentage, p.StateFromCache, p.StateFromAPI, elapsed.Seconds())
}

// CompleteStateRetrieval completes the pipeline state retrieval phase
func (p *DetailedProgressInfo) CompleteStateRetrieval() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.StateEndTime = time.Now()

	elapsed := p.StateEndTime.Sub(p.StateStartTime)
	fmt.Printf("✅ Step 4/4: Retrieved %d pipeline states (Cache: %d, API: %d) (%.2fs)\n",
		p.StateCurrent, p.StateFromCache, p.StateFromAPI, elapsed.Seconds())
}

// CompleteAll completes the entire operation
func (p *DetailedProgressInfo) CompleteAll() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.TotalEndTime = time.Now()

	totalElapsed := p.TotalEndTime.Sub(p.TotalStartTime)
	fmt.Printf("\n🎉 All steps completed in %.2fs\n", totalElapsed.Seconds())

	// Summary
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   • Accounts: %d (from %s)\n", p.AccountsTotal, p.getSourceString(p.AccountsFromCache))
	fmt.Printf("   • Pipelines: %d (from %s)\n", p.PipelinesTotal, p.getSourceString(p.PipelinesFromCache))
	fmt.Printf("   • Details: %d (Cache: %d, API: %d)\n", p.DetailsCurrent, p.DetailsFromCache, p.DetailsFromAPI)
	if p.StateTotal > 0 {
		fmt.Printf("   • States: %d (Cache: %d, API: %d)\n", p.StateCurrent, p.StateFromCache, p.StateFromAPI)
	}
	fmt.Println()
}

// getSourceString returns a human-readable source string
func (p *DetailedProgressInfo) getSourceString(fromCache bool) string {
	if fromCache {
		return "Cache"
	}
	return "API"
}
