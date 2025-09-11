package config

import (
	"os"
	"path/filepath"

	"aft-pipeline-tool/internal/logger"
	"aft-pipeline-tool/internal/models"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	Cache           CacheConfig       `yaml:"cache"`
	ExcludeAccounts []string          `yaml:"exclude_accounts"`
	AWS             AWSConfig         `yaml:"aws"`
	Output          OutputConfig      `yaml:"output"`
	BatchUpdate     BatchUpdateConfig `yaml:"batch_update"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	Directory          string `yaml:"directory"`
	AccountsTTL        int    `yaml:"accounts_ttl"`
	PipelinesTTL       int    `yaml:"pipelines_ttl"`
	PipelineDetailsTTL int    `yaml:"pipeline_details_ttl"`
}

// AWSConfig represents AWS configuration
type AWSConfig struct {
	Region  string `yaml:"region"`
	Profile string `yaml:"profile"`
}

// OutputConfig represents output configuration
type OutputConfig struct {
	Format string `yaml:"format"` // table, json, csv
	Color  bool   `yaml:"color"`
}

// BatchUpdateConfig represents batch update configuration
type BatchUpdateConfig struct {
	TargetAccounts []string               `yaml:"target_accounts"`
	Settings       models.TriggerSettings `yaml:"settings"`
	DryRun         bool                   `yaml:"dry_run"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig() (*Config, error) {
	// Set default values (these will be used if not specified in config file)
	viper.SetDefault("cache.directory", getDefaultCacheDir())
	viper.SetDefault("cache.accounts_ttl", 3600)
	viper.SetDefault("cache.pipelines_ttl", 1800)
	viper.SetDefault("cache.pipeline_details_ttl", 900)
	// AWS設定のデフォルトは空文字列（現在の環境を使用）
	viper.SetDefault("aws.region", "")
	viper.SetDefault("aws.profile", "")
	viper.SetDefault("output.format", "table")
	viper.SetDefault("output.color", true)
	viper.SetDefault("batch_update.dry_run", true)

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Manual override for cache TTL values if viper.Unmarshal didn't work properly
	if config.Cache.AccountsTTL == 0 {
		config.Cache.AccountsTTL = viper.GetInt("cache.accounts_ttl")
	}
	if config.Cache.PipelinesTTL == 0 {
		config.Cache.PipelinesTTL = viper.GetInt("cache.pipelines_ttl")
	}
	if config.Cache.PipelineDetailsTTL == 0 {
		config.Cache.PipelineDetailsTTL = viper.GetInt("cache.pipeline_details_ttl")
	}

	// Expand tilde in cache directory path
	config.Cache.Directory = expandPath(config.Cache.Directory)

	logger.GetLogger().Debug("Configuration loaded",
		zap.String("cache_directory", config.Cache.Directory),
		zap.Int("accounts_ttl", config.Cache.AccountsTTL),
		zap.Int("pipelines_ttl", config.Cache.PipelinesTTL),
		zap.Int("pipeline_details_ttl", config.Cache.PipelineDetailsTTL))

	return &config, nil
}

// getDefaultCacheDir returns the default cache directory
func getDefaultCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".aft-pipeline-tool/cache"
	}
	return filepath.Join(homeDir, ".aft-pipeline-tool", "cache")
}

// expandPath expands tilde (~) in file paths
func expandPath(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if len(path) == 1 {
		return homeDir
	}

	if path[1] == '/' {
		return filepath.Join(homeDir, path[2:])
	}

	return path
}
