package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache management commands",
	Long:  `Manage cache data for AFT pipeline tool.`,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached data",
	Long:  `Clear all cached data including accounts, pipelines, pipeline details, and executions.`,
	RunE:  runCacheClear,
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cache status",
	Long:  `Show the current cache status for all data types.`,
	RunE:  runCacheStatus,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatusCmd)
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	// 設定読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// キャッシュクリア実行
	if err := fileCache.ClearCache(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Printf("Cache cleared successfully from directory: %s\n", cfg.Cache.Directory)
	return nil
}

func runCacheStatus(cmd *cobra.Command, args []string) error {
	// 設定読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	return showCacheStatus(fileCache, cfg, awsContext)
}

func showCacheStatus(fileCache *cache.FileCache, cfg *config.Config, awsContext *cache.AWSContext) error {
	fmt.Printf("=== Cache Status ===\n\n")

	// Cache configuration
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Base Directory: %s\n", cfg.Cache.Directory)
	fmt.Printf("  AWS Region: %s\n", cfg.AWS.Region)
	fmt.Printf("  AWS Profile: %s\n", cfg.AWS.Profile)
	fmt.Printf("  Cache Subdirectory: %s\n", awsContext.GetCacheSubDirectory())
	fmt.Printf("  TTL Settings:\n")
	fmt.Printf("    Accounts: %d seconds\n", cfg.Cache.AccountsTTL)
	fmt.Printf("    Pipelines: %d seconds\n", cfg.Cache.PipelinesTTL)
	fmt.Printf("    Pipeline Details: %d seconds\n", cfg.Cache.PipelineDetailsTTL)
	fmt.Println()

	// Cache status
	fmt.Printf("Cache Data Status:\n")

	// Accounts cache
	if accountsCache, err := fileCache.GetAccounts(); err == nil {
		elapsed := time.Since(accountsCache.CachedAt)
		remaining := time.Duration(accountsCache.TTL)*time.Second - elapsed
		status := "Valid"
		if remaining <= 0 {
			status = "Expired"
		}
		fmt.Printf("  ✓ Accounts: %s (%d items, cached %s ago, %s)\n",
			status, len(accountsCache.Accounts), formatDuration(elapsed), formatRemaining(remaining))
	} else {
		fmt.Printf("  ✗ Accounts: Not available\n")
	}

	// Pipelines cache
	if pipelinesCache, err := fileCache.GetPipelines(); err == nil {
		elapsed := time.Since(pipelinesCache.CachedAt)
		remaining := time.Duration(pipelinesCache.TTL)*time.Second - elapsed
		status := "Valid"
		if remaining <= 0 {
			status = "Expired"
		}
		fmt.Printf("  ✓ Pipelines: %s (%d items, cached %s ago, %s)\n",
			status, len(pipelinesCache.Pipelines), formatDuration(elapsed), formatRemaining(remaining))
	} else {
		fmt.Printf("  ✗ Pipelines: Not available\n")
	}

	// Check individual pipeline caches
	fmt.Printf("  Pipeline Details:\n")
	detailCount, stateCount, execCount := countPipelineCaches(fileCache)
	if detailCount > 0 {
		fmt.Printf("    ✓ Details: %d pipelines cached\n", detailCount)
	} else {
		fmt.Printf("    ✗ Details: No cached pipeline details\n")
	}

	if stateCount > 0 {
		fmt.Printf("    ✓ States: %d pipelines cached\n", stateCount)
	} else {
		fmt.Printf("    ✗ States: No cached pipeline states\n")
	}

	if execCount > 0 {
		fmt.Printf("    ✓ Executions: %d pipelines cached\n", execCount)
	} else {
		fmt.Printf("    ✗ Executions: No cached pipeline executions\n")
	}

	// Cache size information
	fmt.Println()
	fmt.Printf("Cache Size Information:\n")
	totalSize, err := calculateCacheSize(cfg.Cache.Directory, awsContext)
	if err != nil {
		fmt.Printf("  Unable to calculate cache size: %v\n", err)
	} else {
		fmt.Printf("  Total Size: %s\n", formatBytes(totalSize))
	}

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
}

func formatRemaining(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	return fmt.Sprintf("expires in %s", formatDuration(d))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func countPipelineCaches(fileCache *cache.FileCache) (int, int, int) {
	// Get the cache directory path
	cacheDir := getCacheDirFromFileCache(fileCache)

	detailCount := countFilesInDir(filepath.Join(cacheDir, "pipeline_details"))
	stateCount := countFilesInDir(filepath.Join(cacheDir, "pipeline_states"))
	execCount := countFilesInDir(filepath.Join(cacheDir, "pipeline_executions"))

	return detailCount, stateCount, execCount
}

func getCacheDirFromFileCache(fileCache *cache.FileCache) string {
	return fileCache.GetCacheDir()
}

func countFilesInDir(dirPath string) int {
	if dirPath == "" {
		return 0
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}

	return count
}

func calculateCacheSize(baseDir string, awsContext *cache.AWSContext) (int64, error) {
	cacheDir := filepath.Join(baseDir, awsContext.GetCacheSubDirectory())

	var totalSize int64
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files that can't be accessed
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}

	return totalSize, nil
}
