package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/logger"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"

	"go.uber.org/zap"
)

// FileCache represents a file-based cache implementation
type FileCache struct {
	baseDir    string
	awsContext *AWSContext
	config     *CacheConfig
}

// NewFileCache creates a new file cache instance
func NewFileCache(baseDir string) *FileCache {
	return &FileCache{
		baseDir:    baseDir,
		awsContext: nil, // Will be set when AWS context is available
	}
}

// NewFileCacheWithContext creates a new file cache instance with AWS context
func NewFileCacheWithContext(baseDir string, awsContext *AWSContext) *FileCache {
	return &FileCache{
		baseDir:    baseDir,
		awsContext: awsContext,
	}
}

// SetAWSContext sets the AWS context for cache isolation
func (fc *FileCache) SetAWSContext(awsContext *AWSContext) {
	fc.awsContext = awsContext
}

// getCacheDir returns the cache directory with AWS context isolation
func (fc *FileCache) getCacheDir() string {
	if fc.awsContext == nil {
		// Fallback to legacy behavior for backward compatibility
		return fc.baseDir
	}
	return filepath.Join(fc.baseDir, fc.awsContext.GetCacheSubDirectory())
}

// GetCacheDir returns the cache directory (public method for external access)
func (fc *FileCache) GetCacheDir() string {
	return fc.getCacheDir()
}

// CacheResult represents the result of a cache operation
type CacheResult struct {
	Data      interface{}
	FromCache bool
}

// GetAccounts retrieves cached account data
func (fc *FileCache) GetAccounts() (*models.AccountsCache, error) {
	var cache models.AccountsCache
	if err := fc.getCacheItem("accounts.json", &cache); err != nil {
		return nil, err
	}

	// TTLチェック
	if err := checkTTL(cache.CachedAt, cache.TTL); err != nil {
		logger.GetLogger().Debug("Cache expired")
		return nil, err
	}

	logger.GetLogger().Debug("Cache hit - returning accounts", zap.Int("count", len(cache.Accounts)))
	return &cache, nil
}

// SetAccounts stores account data in cache
func (fc *FileCache) SetAccounts(accounts []models.Account, ttl int) error {
	cache := models.AccountsCache{
		Accounts: accounts,
		CachedAt: time.Now(),
		TTL:      ttl,
	}

	return fc.setCacheItem("accounts.json", cache)
}

// GetPipelines retrieves cached pipeline data
func (fc *FileCache) GetPipelines() (*models.PipelinesCache, error) {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipelines.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache models.PipelinesCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	// TTLチェック
	if time.Since(cache.CachedAt).Seconds() > float64(cache.TTL) {
		return nil, fmt.Errorf("cache expired")
	}

	return &cache, nil
}

// SetPipelines stores pipeline data in cache
func (fc *FileCache) SetPipelines(pipelines []models.Pipeline, ttl int) error {
	cache := models.PipelinesCache{
		Pipelines: pipelines,
		CachedAt:  time.Now(),
		TTL:       ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipelines.json")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// GetPipelineDetail retrieves cached individual pipeline detail data
func (fc *FileCache) GetPipelineDetail(pipelineName string) (*models.PipelineDetailCache, error) {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipeline_details", fmt.Sprintf("%s.json", pipelineName))

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read pipeline detail cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.PipelineDetailCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal pipeline detail cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Pipeline detail cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL), zap.String("pipeline", pipelineName))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Pipeline detail cache expired", zap.String("pipeline", pipelineName))
		return nil, fmt.Errorf("cache expired")
	}

	logger.GetLogger().Debug("Pipeline detail cache hit", zap.String("pipeline", pipelineName))
	return &cache, nil
}

// SetPipelineDetail stores individual pipeline detail data in cache
func (fc *FileCache) SetPipelineDetail(pipeline models.Pipeline, ttl int) error {
	cache := models.PipelineDetailCache{
		Pipeline: pipeline,
		CachedAt: time.Now(),
		TTL:      ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := fc.getCacheDir()
	pipelineDetailsDir := filepath.Join(cacheDir, "pipeline_details")
	if err := os.MkdirAll(pipelineDetailsDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(pipelineDetailsDir, fmt.Sprintf("%s.json", pipeline.GetName()))
	return os.WriteFile(cachePath, data, 0644)
}

// GetPipelineState retrieves cached pipeline state data
func (fc *FileCache) GetPipelineState(pipelineName string) (*models.PipelineStateCache, error) {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipeline_states", fmt.Sprintf("%s.json", pipelineName))

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read pipeline state cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.PipelineStateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal pipeline state cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Pipeline state cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL), zap.String("pipeline", pipelineName))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Pipeline state cache expired", zap.String("pipeline", pipelineName))
		return nil, fmt.Errorf("cache expired")
	}

	logger.GetLogger().Debug("Pipeline state cache hit", zap.String("pipeline", pipelineName))
	return &cache, nil
}

// SetPipelineState stores individual pipeline state data in cache
func (fc *FileCache) SetPipelineState(pipelineName string, state *models.PipelineState, ttl int) error {
	cache := models.PipelineStateCache{
		State:    state,
		CachedAt: time.Now(),
		TTL:      ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := fc.getCacheDir()
	pipelineStatesDir := filepath.Join(cacheDir, "pipeline_states")
	if err := os.MkdirAll(pipelineStatesDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(pipelineStatesDir, fmt.Sprintf("%s.json", pipelineName))
	return os.WriteFile(cachePath, data, 0644)
}

// GetPipelineExecutions retrieves cached pipeline execution data
func (fc *FileCache) GetPipelineExecutions(pipelineName string) (*models.PipelineExecutionsCache, error) {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipeline_executions", fmt.Sprintf("%s.json", pipelineName))

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read pipeline execution cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.PipelineExecutionsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal pipeline execution cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Pipeline execution cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL), zap.String("pipeline", pipelineName))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Pipeline execution cache expired", zap.String("pipeline", pipelineName))
		return nil, fmt.Errorf("cache expired")
	}

	logger.GetLogger().Debug("Pipeline execution cache hit", zap.String("pipeline", pipelineName))
	return &cache, nil
}

// SetPipelineExecutions stores pipeline execution data in cache
func (fc *FileCache) SetPipelineExecutions(pipelineName string, executions []models.PipelineExecutionSummary, ttl int) error {
	cache := models.PipelineExecutionsCache{
		Executions: executions,
		CachedAt:   time.Now(),
		TTL:        ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := fc.getCacheDir()
	pipelineExecutionsDir := filepath.Join(cacheDir, "pipeline_executions")
	if err := os.MkdirAll(pipelineExecutionsDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(pipelineExecutionsDir, fmt.Sprintf("%s.json", pipelineName))
	return os.WriteFile(cachePath, data, 0644)
}

// ClearCache removes all cached data
func (fc *FileCache) ClearCache() error {
	var errors []error

	// Legacy cache directories to clean up (in base directory for backward compatibility)
	legacyPipelineDetailsPath := filepath.Join(fc.baseDir, "pipeline_details.json")
	legacyPipelineDetailsWithStateDirPath := filepath.Join(fc.baseDir, "pipeline_details_with_state")

	// Remove legacy files first
	if err := os.Remove(legacyPipelineDetailsPath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove legacy pipeline details cache", zap.Error(err))
		errors = append(errors, fmt.Errorf("failed to remove legacy pipeline details cache: %w", err))
	}
	if err := os.RemoveAll(legacyPipelineDetailsWithStateDirPath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove legacy pipeline details with state directory", zap.Error(err))
		errors = append(errors, fmt.Errorf("failed to remove legacy pipeline details with state directory: %w", err))
	}

	// Remove all AWS context-specific directories and files in the base cache directory
	logger.GetLogger().Debug("Clearing all cache data", zap.String("baseDir", fc.baseDir))

	// Read all entries in the base cache directory
	entries, err := os.ReadDir(fc.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.GetLogger().Debug("Cache directory does not exist", zap.String("dir", fc.baseDir))
			return nil // Nothing to clear
		}
		logger.GetLogger().Error("Failed to read cache directory", zap.String("dir", fc.baseDir), zap.Error(err))
		return fmt.Errorf("failed to read cache directory %s: %w", fc.baseDir, err)
	}

	// Remove all entries in the cache directory
	for _, entry := range entries {
		entryPath := filepath.Join(fc.baseDir, entry.Name())
		logger.GetLogger().Debug("Removing cache entry", zap.String("path", entryPath), zap.Bool("isDir", entry.IsDir()))

		if entry.IsDir() {
			// Remove directory and all its contents
			if err := os.RemoveAll(entryPath); err != nil {
				logger.GetLogger().Error("Failed to remove cache directory", zap.String("path", entryPath), zap.Error(err))
				errors = append(errors, fmt.Errorf("failed to remove cache directory %s: %w", entryPath, err))
			} else {
				logger.GetLogger().Debug("Successfully removed cache directory", zap.String("path", entryPath))
			}
		} else {
			// Remove file
			if err := os.Remove(entryPath); err != nil {
				logger.GetLogger().Error("Failed to remove cache file", zap.String("path", entryPath), zap.Error(err))
				errors = append(errors, fmt.Errorf("failed to remove cache file %s: %w", entryPath, err))
			} else {
				logger.GetLogger().Debug("Successfully removed cache file", zap.String("path", entryPath))
			}
		}
	}

	// Return combined errors if any occurred
	if len(errors) > 0 {
		var errorMessages []string
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("cache clear encountered errors: %s", strings.Join(errorMessages, "; "))
	}

	logger.GetLogger().Debug("Cache cleared successfully")
	return nil
}

// DeletePipelineCache removes cached data for a specific pipeline
func (fc *FileCache) DeletePipelineCache(pipelineName string) error {
	cacheDir := fc.getCacheDir()

	// Delete individual pipeline detail cache
	cachePath := filepath.Join(cacheDir, "pipeline_details", fmt.Sprintf("%s.json", pipelineName))
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove pipeline detail cache", zap.String("pipeline", pipelineName), zap.Error(err))
		return fmt.Errorf("failed to remove pipeline detail cache for %s: %w", pipelineName, err)
	}

	// Delete individual pipeline state cache
	stateCachePath := filepath.Join(cacheDir, "pipeline_states", fmt.Sprintf("%s.json", pipelineName))
	if err := os.Remove(stateCachePath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove pipeline state cache", zap.String("pipeline", pipelineName), zap.Error(err))
		// Don't return error here as this is not critical
	}

	// Delete individual pipeline execution cache
	executionCachePath := filepath.Join(cacheDir, "pipeline_executions", fmt.Sprintf("%s.json", pipelineName))
	if err := os.Remove(executionCachePath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove pipeline execution cache", zap.String("pipeline", pipelineName), zap.Error(err))
		// Don't return error here as this is not critical
	}

	// Update pipelines.json to remove the specific pipeline
	if err := fc.removePipelineFromPipelinesCache(pipelineName); err != nil {
		logger.GetLogger().Debug("Failed to remove pipeline from pipelines cache", zap.String("pipeline", pipelineName), zap.Error(err))
		// Don't return error here as this is not critical
	}

	logger.GetLogger().Debug("Pipeline cache deleted successfully", zap.String("pipeline", pipelineName))
	return nil
}

// removePipelineFromPipelinesCache removes a specific pipeline from pipelines.json cache
func (fc *FileCache) removePipelineFromPipelinesCache(pipelineName string) error {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "pipelines.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Cache file doesn't exist, nothing to remove
		}
		return err
	}

	var cache models.PipelinesCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	// Filter out the specific pipeline
	var updatedPipelines []models.Pipeline
	for _, pipeline := range cache.Pipelines {
		if pipeline.GetName() != pipelineName {
			updatedPipelines = append(updatedPipelines, pipeline)
		}
	}

	// Update cache with filtered pipelines
	cache.Pipelines = updatedPipelines

	// Write back to file
	updatedData, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, updatedData, 0644)
}

// Generic cache operations implementation

// Get retrieves cache data for a given key
func (fc *FileCache) Get(key string, target interface{}) error {
	return fc.getCacheItem(key, target)
}

// Set stores cache data for a given key
func (fc *FileCache) Set(key string, data interface{}, ttl int) error {
	cacheItem := NewCacheItem(data, ttl)
	return fc.setCacheItem(key, cacheItem)
}

// Delete removes cache data for a given key
func (fc *FileCache) Delete(key string) error {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, key)

	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return &CacheError{Operation: "delete", Key: key, Err: err}
	}

	return nil
}

// Exists checks if cache data exists for a given key
func (fc *FileCache) Exists(key string) bool {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, key)

	if _, err := os.Stat(cachePath); err != nil {
		return false
	}

	return true
}

// GetMultiple retrieves multiple cache items
func (fc *FileCache) GetMultiple(keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, key := range keys {
		var item CacheItem
		if err := fc.getCacheItem(key, &item); err != nil {
			// Skip missing items, don't return error
			continue
		}

		// Check TTL
		if !item.IsExpired() {
			result[key] = item.Data
		}
	}

	return result, nil
}

// SetMultiple stores multiple cache items
func (fc *FileCache) SetMultiple(items map[string]interface{}, ttl int) error {
	var errors []error

	for key, data := range items {
		if err := fc.Set(key, data, ttl); err != nil {
			errors = append(errors, fmt.Errorf("failed to set %s: %w", key, err))
		}
	}

	if len(errors) > 0 {
		var errorMessages []string
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("batch set encountered errors: %s", strings.Join(errorMessages, "; "))
	}

	return nil
}
