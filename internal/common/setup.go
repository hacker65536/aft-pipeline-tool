package common

import (
	"context"
	"fmt"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

// CommonSetup contains all the common components needed by fix commands
type CommonSetup struct {
	Config    *config.Config
	AWSClient *aws.Client
	FileCache *cache.FileCache
	Manager   *aft.Manager
}

// NewCommonSetup initializes all common components needed by fix commands
// This function consolidates the initialization logic that was duplicated across
// fix_connections.go, fix_triggers.go, and fix_pipeline_type.go
func NewCommonSetup(ctx context.Context) (*CommonSetup, error) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize AWS client
	awsClient, err := aws.NewClient(ctx, cfg.AWS.Region, cfg.AWS.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Initialize cache with AWS context
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// Initialize AFT manager
	manager := aft.NewManager(awsClient, fileCache, cfg)

	return &CommonSetup{
		Config:    cfg,
		AWSClient: awsClient,
		FileCache: fileCache,
		Manager:   manager,
	}, nil
}
