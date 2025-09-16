package cmd

import (
	"fmt"

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

	fmt.Printf("Cache Directory: %s\n", cfg.Cache.Directory)
	fmt.Println()

	// 各キャッシュファイルの存在確認
	fmt.Println("Cache Status:")

	// Accounts cache
	if _, err := fileCache.GetAccounts(); err == nil {
		fmt.Printf("  Accounts: Available\n")
	} else {
		fmt.Printf("  Accounts: Not available\n")
	}

	// Pipelines cache
	if _, err := fileCache.GetPipelines(); err == nil {
		fmt.Printf("  Pipelines: Available\n")
	} else {
		fmt.Printf("  Pipelines: Not available\n")
	}

	// Pipeline details cache (check if any individual pipeline details exist)
	fmt.Printf("  Pipeline Details: Check individual pipelines\n")

	// Pipeline executions cache (check if any individual pipeline executions exist)
	fmt.Printf("  Pipeline Executions: Check individual pipelines\n")

	return nil
}
