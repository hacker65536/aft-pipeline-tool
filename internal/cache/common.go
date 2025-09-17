package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/logger"
	"go.uber.org/zap"
)

// CacheConfig holds configuration for cache operations
type CacheConfig struct {
	DirPermission  os.FileMode
	FilePermission os.FileMode
	DefaultTTL     int
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DirPermission:  0755,
		FilePermission: 0644,
		DefaultTTL:     3600, // 1 hour
	}
}

// CacheError represents a structured cache error
type CacheError struct {
	Operation string
	Key       string
	Err       error
}

func (e *CacheError) Error() string {
	return fmt.Sprintf("cache %s failed for key %s: %v", e.Operation, e.Key, e.Err)
}

// checkTTL validates if cache item has expired
func checkTTL(cachedAt time.Time, ttl int) error {
	elapsed := time.Since(cachedAt).Seconds()
	if elapsed > float64(ttl) {
		return fmt.Errorf("cache expired")
	}
	return nil
}

// getCacheItem reads and unmarshals cache data from file
func (fc *FileCache) getCacheItem(filename string, target interface{}) error {
	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, filename)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		logger.GetLogger().Debug("Failed to read cache file", zap.String("path", cachePath), zap.Error(err))
		return &CacheError{Operation: "read", Key: filename, Err: err}
	}

	if err := json.Unmarshal(data, target); err != nil {
		logger.GetLogger().Debug("Failed to unmarshal cache data", zap.String("file", filename), zap.Error(err))
		return &CacheError{Operation: "unmarshal", Key: filename, Err: err}
	}

	return nil
}

// setCacheItem marshals and writes cache data to file
func (fc *FileCache) setCacheItem(filename string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return &CacheError{Operation: "marshal", Key: filename, Err: err}
	}

	cacheDir := fc.getCacheDir()
	cachePath := filepath.Join(cacheDir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), fc.getConfig().DirPermission); err != nil {
		return &CacheError{Operation: "mkdir", Key: filename, Err: err}
	}

	if err := os.WriteFile(cachePath, jsonData, fc.getConfig().FilePermission); err != nil {
		return &CacheError{Operation: "write", Key: filename, Err: err}
	}

	return nil
}

// getConfig returns cache configuration (with defaults if not set)
func (fc *FileCache) getConfig() *CacheConfig {
	if fc.config == nil {
		fc.config = DefaultCacheConfig()
	}
	return fc.config
}

// Generic cache operations using Go generics (requires Go 1.18+)

// GetCacheData retrieves and unmarshals cache data with type safety
func GetCacheData[T any](fc *FileCache, filename string) (*T, error) {
	var result T
	if err := fc.getCacheItem(filename, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetCacheData marshals and stores cache data with type safety
func SetCacheData[T any](fc *FileCache, filename string, data T) error {
	return fc.setCacheItem(filename, data)
}

// CacheItemWithTTL interface for items that have TTL functionality
type CacheItemWithTTL interface {
	GetCachedAt() time.Time
	GetTTL() int
}

// ValidateCacheItem checks if a cache item is still valid (not expired)
func ValidateCacheItem(item CacheItemWithTTL) error {
	return checkTTL(item.GetCachedAt(), item.GetTTL())
}
