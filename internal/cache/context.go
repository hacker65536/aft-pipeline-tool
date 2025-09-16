package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

// AWSContext represents AWS connection information for cache isolation
type AWSContext struct {
	Region  string
	Profile string
	UserID  string // AWS IAM User ID (kept for backward compatibility)
	Account string // AWS Account ID (kept for backward compatibility)
}

// GenerateCacheKey generates a unique cache key based on AWS Profile
func (ctx *AWSContext) GenerateCacheKey() string {
	// Get effective profile (prioritize AWS_PROFILE environment variable)
	effectiveProfile := ctx.getEffectiveProfile()

	// Primary key based on AWS Profile and Region
	var key string
	key = fmt.Sprintf("region:%s|profile:%s", ctx.Region, effectiveProfile)

	// Add AWS credential information if available for additional uniqueness
	if accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID"); accessKeyID != "" {
		// Use only first 8 characters for privacy and uniqueness
		if len(accessKeyID) > 8 {
			key += "|access_key:" + accessKeyID[:8]
		} else {
			key += "|access_key:" + accessKeyID
		}
	}

	// Generate a hash of the key for consistent directory naming
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 characters of hash
}

// getEffectiveProfile returns the effective AWS profile, prioritizing AWS_PROFILE environment variable
func (ctx *AWSContext) getEffectiveProfile() string {
	// Check AWS_PROFILE environment variable first
	if envProfile := os.Getenv("AWS_PROFILE"); envProfile != "" {
		return envProfile
	}

	// Fall back to the profile set in the context
	return ctx.Profile
}

// GetCacheSubDirectory returns the subdirectory name for this AWS context
func (ctx *AWSContext) GetCacheSubDirectory() string {
	cacheKey := ctx.GenerateCacheKey()

	// Create a human-readable directory name with the hash
	var parts []string

	// Primary directory naming based on AWS Profile and Region
	if ctx.Region != "" {
		parts = append(parts, "r-"+ctx.Region)
	}

	// Use effective profile (prioritizing AWS_PROFILE environment variable)
	effectiveProfile := ctx.getEffectiveProfile()
	if effectiveProfile != "" {
		parts = append(parts, "p-"+effectiveProfile)
	}

	if len(parts) > 0 {
		return strings.Join(parts, "_") + "_" + cacheKey
	}

	return "default_" + cacheKey
}

// NewAWSContext creates a new AWS context
func NewAWSContext(region, profile string) *AWSContext {
	return &AWSContext{
		Region:  region,
		Profile: profile,
	}
}

// NewAWSContextWithIdentity creates a new AWS context with IAM identity information
func NewAWSContextWithIdentity(region, profile, userID, account string) *AWSContext {
	return &AWSContext{
		Region:  region,
		Profile: profile,
		UserID:  userID,
		Account: account,
	}
}
