package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/logger"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"

	"go.uber.org/zap"
)

// FileCache represents a file-based cache implementation
type FileCache struct {
	baseDir    string
	awsContext *AWSContext
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

// CacheResult represents the result of a cache operation
type CacheResult struct {
	Data      interface{}
	FromCache bool
}

// GetAccounts retrieves cached account data
func (fc *FileCache) GetAccounts() (*models.AccountsCache, error) {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "accounts.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.AccountsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Cache expired")
		return nil, fmt.Errorf("cache expired")
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

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, "accounts.json")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
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
	cacheDir := fc.getCacheDir()

	accountsPath := filepath.Join(cacheDir, "accounts.json")
	pipelinesPath := filepath.Join(cacheDir, "pipelines.json")
	pipelineDetailsDirPath := filepath.Join(cacheDir, "pipeline_details")
	pipelineStatesDirPath := filepath.Join(cacheDir, "pipeline_states")
	pipelineExecutionsDirPath := filepath.Join(cacheDir, "pipeline_executions")

	// Legacy cache directories to clean up (in base directory for backward compatibility)
	legacyPipelineDetailsPath := filepath.Join(fc.baseDir, "pipeline_details.json")
	legacyPipelineDetailsWithStateDirPath := filepath.Join(fc.baseDir, "pipeline_details_with_state")

	// Remove files if they exist, ignore errors if files don't exist
	if err := os.Remove(accountsPath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove accounts cache", zap.Error(err))
	}
	if err := os.Remove(pipelinesPath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove pipelines cache", zap.Error(err))
	}
	if err := os.Remove(legacyPipelineDetailsPath); err != nil && !os.IsNotExist(err) {
		logger.GetLogger().Debug("Failed to remove legacy pipeline details cache", zap.Error(err))
	}
	if err := os.RemoveAll(pipelineDetailsDirPath); err != nil {
		logger.GetLogger().Debug("Failed to remove pipeline details directory", zap.Error(err))
	}
	if err := os.RemoveAll(pipelineStatesDirPath); err != nil {
		logger.GetLogger().Debug("Failed to remove pipeline states directory", zap.Error(err))
	}
	if err := os.RemoveAll(pipelineExecutionsDirPath); err != nil {
		logger.GetLogger().Debug("Failed to remove pipeline executions directory", zap.Error(err))
	}
	if err := os.RemoveAll(legacyPipelineDetailsWithStateDirPath); err != nil {
		logger.GetLogger().Debug("Failed to remove legacy pipeline details with state directory", zap.Error(err))
	}

	// If AWS context is available, also remove the entire context-specific directory
	if fc.awsContext != nil {
		if err := os.RemoveAll(cacheDir); err != nil {
			logger.GetLogger().Debug("Failed to remove AWS context cache directory", zap.String("dir", cacheDir), zap.Error(err))
		}
	}

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

// Legacy methods for backward compatibility - these will be deprecated

// GetPipelineDetails retrieves cached pipeline details data (legacy)
// Deprecated: Use GetPipelineDetail for individual pipelines instead
func (fc *FileCache) GetPipelineDetails() (*models.PipelineDetailsCache, error) {
	cachePath := filepath.Join(fc.baseDir, "pipeline_details.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read pipeline details cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.PipelineDetailsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal pipeline details cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Pipeline details cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Pipeline details cache expired")
		return nil, fmt.Errorf("cache expired")
	}

	logger.GetLogger().Debug("Pipeline details cache hit - returning pipelines", zap.Int("count", len(cache.Pipelines)))
	return &cache, nil
}

// SetPipelineDetails stores pipeline details data in cache (legacy)
// Deprecated: Use SetPipelineDetail for individual pipelines instead
func (fc *FileCache) SetPipelineDetails(pipelines []models.Pipeline, ttl int) error {
	cache := models.PipelineDetailsCache{
		Pipelines: pipelines,
		CachedAt:  time.Now(),
		TTL:       ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cachePath := filepath.Join(fc.baseDir, "pipeline_details.json")
	if err := os.MkdirAll(fc.baseDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// GetPipelineDetailWithState retrieves cached individual pipeline detail data with state information (legacy)
// Deprecated: Use GetPipelineDetail and GetPipelineState separately instead
func (fc *FileCache) GetPipelineDetailWithState(pipelineName string) (*models.PipelineDetailCache, error) {
	cachePath := filepath.Join(fc.baseDir, "pipeline_details_with_state", fmt.Sprintf("%s.json", pipelineName))

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read pipeline detail with state cache file", zap.String("path", cachePath), zap.Error(err))
		return nil, err
	}

	var cache models.PipelineDetailCache
	if err := json.Unmarshal(data, &cache); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal pipeline detail with state cache data", zap.Error(err))
		return nil, err
	}

	// TTLチェック
	elapsed := time.Since(cache.CachedAt).Seconds()
	logger.GetLogger().Debug("Pipeline detail with state cache TTL check", zap.Float64("elapsed", elapsed), zap.Int("ttl", cache.TTL), zap.String("pipeline", pipelineName))
	if elapsed > float64(cache.TTL) {
		logger.GetLogger().Debug("Pipeline detail with state cache expired", zap.String("pipeline", pipelineName))
		return nil, fmt.Errorf("cache expired")
	}

	logger.GetLogger().Debug("Pipeline detail with state cache hit", zap.String("pipeline", pipelineName))
	return &cache, nil
}

// SetPipelineDetailWithState stores individual pipeline detail data with state information in cache (legacy)
// Deprecated: Use SetPipelineDetail and SetPipelineState separately instead
func (fc *FileCache) SetPipelineDetailWithState(pipeline models.Pipeline, ttl int) error {
	cache := models.PipelineDetailCache{
		Pipeline: pipeline,
		CachedAt: time.Now(),
		TTL:      ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(fc.baseDir, "pipeline_details_with_state")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s.json", pipeline.GetName()))
	return os.WriteFile(cachePath, data, 0644)
}
