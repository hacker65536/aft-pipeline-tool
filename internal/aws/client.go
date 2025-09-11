package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

// Client represents AWS service clients
type Client struct {
	Organizations *organizations.Client
	CodePipeline  *codepipeline.Client
	Config        awsconfig.Config
}

// NewClient creates a new AWS client with the specified region and profile
// If region or profile are empty strings, the current environment settings will be used
func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	// Only set region if explicitly provided (not empty string)
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	// Only set profile if explicitly provided (not empty string)
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	// LoadDefaultConfig will use environment variables, shared config files,
	// and other default sources when no explicit options are provided
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		Organizations: organizations.NewFromConfig(cfg),
		CodePipeline:  codepipeline.NewFromConfig(cfg),
		Config:        cfg,
	}, nil
}
