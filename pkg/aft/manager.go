package aft

import (
	"context"
	"fmt"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/logger"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"go.uber.org/zap"
)

// Manager represents the AFT pipeline manager
type Manager struct {
	awsClient  *aws.Client
	cache      *cache.FileCache
	config     *config.Config
	cacheUsage *CacheUsage
}

// CacheUsage tracks cache usage information
type CacheUsage struct {
	AccountsFromCache        bool
	PipelinesFromCache       bool
	PipelineDetailsFromCache bool
	PipelineStatesFromCache  bool
	ExecutionsFromCache      bool
	accountsStatusSet        bool // 内部フラグ：アカウントキャッシュ状況が設定済みかどうか
}

// NewManager creates a new AFT manager instance
func NewManager(awsClient *aws.Client, cache *cache.FileCache, cfg *config.Config) *Manager {
	return &Manager{
		awsClient:  awsClient,
		cache:      cache,
		config:     cfg,
		cacheUsage: &CacheUsage{},
	}
}

// GetAccounts retrieves accounts from cache or AWS API
func (m *Manager) GetAccounts(ctx context.Context) ([]models.Account, error) {
	logger.GetLogger().Debug("GetAccounts called")

	// 初回呼び出し時のみキャッシュ使用状況を設定
	// 既に設定されている場合は変更しない
	if !m.hasAccountsCacheStatusSet() {
		// キャッシュから取得を試行
		if cachedAccounts, err := m.cache.GetAccounts(); err == nil {
			logger.GetLogger().Debug("Cache hit in GetAccounts")
			m.cacheUsage.AccountsFromCache = true
			m.cacheUsage.accountsStatusSet = true
			return cachedAccounts.Accounts, nil
		} else {
			logger.GetLogger().Debug("Cache miss in GetAccounts", zap.Error(err))
			m.cacheUsage.AccountsFromCache = false
			m.cacheUsage.accountsStatusSet = true
		}
	} else {
		// 既にキャッシュ使用状況が設定されている場合、キャッシュから取得を試行
		if cachedAccounts, err := m.cache.GetAccounts(); err == nil {
			logger.GetLogger().Debug("Cache hit in GetAccounts (subsequent call)")
			return cachedAccounts.Accounts, nil
		} else {
			logger.GetLogger().Debug("Cache miss in GetAccounts (subsequent call)", zap.Error(err))
		}
	}

	// AWS APIから取得
	logger.GetLogger().Debug("Fetching accounts from AWS API")
	accounts, err := m.awsClient.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	// 除外アカウントをフィルタリング
	filteredAccounts := m.awsClient.FilterAFTAccounts(accounts, m.config.ExcludeAccounts)

	// キャッシュに保存
	if err := m.cache.SetAccounts(filteredAccounts, m.config.Cache.AccountsTTL); err != nil {
		// ログ出力のみ、エラーは返さない
		logger.GetLogger().Warn("Failed to cache accounts", zap.Error(err))
	}

	return filteredAccounts, nil
}

// GetPipelines retrieves pipelines from cache or AWS API
func (m *Manager) GetPipelines(ctx context.Context) ([]models.Pipeline, error) {
	logger.GetLogger().Debug("GetPipelines called")
	// キャッシュから取得を試行
	if cachedPipelines, err := m.cache.GetPipelines(); err == nil {
		logger.GetLogger().Debug("Pipeline cache hit")
		m.cacheUsage.PipelinesFromCache = true
		// パイプラインがキャッシュから取得された場合、アカウント情報のキャッシュ状況も確認
		// ただし、既にアカウントキャッシュ状況が設定されている場合は変更しない
		if !m.hasAccountsCacheStatusSet() {
			if _, err := m.cache.GetAccounts(); err == nil {
				m.cacheUsage.AccountsFromCache = true
				m.cacheUsage.accountsStatusSet = true
			} else {
				m.cacheUsage.AccountsFromCache = false
				m.cacheUsage.accountsStatusSet = true
			}
		}
		return cachedPipelines.Pipelines, nil
	} else {
		logger.GetLogger().Debug("Pipeline cache miss", zap.Error(err))
	}

	// パイプラインがキャッシュにない場合
	logger.GetLogger().Debug("Pipeline cache miss, fetching from API")
	m.cacheUsage.PipelinesFromCache = false

	// アカウント一覧を取得（キャッシュ使用状況は GetAccounts 内で設定される）
	logger.GetLogger().Debug("About to call GetAccounts")
	accounts, err := m.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	logger.GetLogger().Debug("GetAccounts returned accounts", zap.Int("count", len(accounts)))

	// AWS APIからパイプライン一覧を取得
	pipelines, err := m.awsClient.ListAFTPipelines(ctx, accounts)
	if err != nil {
		return nil, err
	}

	// キャッシュに保存
	if err := m.cache.SetPipelines(pipelines, m.config.Cache.PipelinesTTL); err != nil {
		logger.GetLogger().Warn("Failed to cache pipelines", zap.Error(err))
	}

	return pipelines, nil
}

// GetPipelineDetails retrieves detailed pipeline information
func (m *Manager) GetPipelineDetails(ctx context.Context) ([]models.Pipeline, error) {
	return m.GetPipelineDetailsWithProgress(ctx, true)
}

// GetPipelineDetailsWithProgress retrieves detailed pipeline information with optional progress display
func (m *Manager) GetPipelineDetailsWithProgress(ctx context.Context, showProgress bool) ([]models.Pipeline, error) {
	logger.GetLogger().Debug("GetPipelineDetails called")

	// パイプライン一覧を取得
	pipelines, err := m.GetPipelines(ctx)
	if err != nil {
		return nil, err
	}

	// アカウント情報を取得してIDから名前へのマッピングを作成
	// ここでは既に GetPipelines 内で GetAccounts が呼ばれているため、
	// キャッシュ使用状況は既に設定されている
	accounts, err := m.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}

	accountNameMap := make(map[string]string)
	for _, account := range accounts {
		accountNameMap[account.ID] = account.Name
	}

	// 進捗情報を初期化
	var progress *models.ProgressInfo
	if showProgress && len(pipelines) > 0 {
		progress = models.NewProgressInfo(len(pipelines))
		fmt.Printf("Retrieving pipeline details for %d pipelines...\n", len(pipelines))
	}

	var detailedPipelines []models.Pipeline
	var pipelinesFromCache int
	var pipelinesFromAPI int

	logger.GetLogger().Debug("Checking individual pipeline caches", zap.Int("total_pipelines", len(pipelines)))

	for i, pipeline := range pipelines {
		// 個別のパイプラインキャッシュをチェック
		if cachedDetail, err := m.cache.GetPipelineDetail(pipeline.GetName()); err == nil {
			logger.GetLogger().Debug("Pipeline detail cache hit", zap.String("pipeline", pipeline.GetName()))

			// キャッシュされたパイプラインにAccountNameが設定されていない場合は設定する
			cachedPipeline := cachedDetail.Pipeline
			if cachedPipeline.AccountName == "" {
				cachedPipeline.AccountName = accountNameMap[cachedPipeline.AccountID]
			}

			detailedPipelines = append(detailedPipelines, cachedPipeline)
			pipelinesFromCache++

			if progress != nil {
				progress.IncrementCache()
			}
		} else {
			logger.GetLogger().Debug("Pipeline detail cache miss, fetching from API", zap.String("pipeline", pipeline.GetName()), zap.Error(err))

			// APIから詳細情報を取得
			detailed, err := m.awsClient.GetPipelineDetails(ctx, pipeline.GetName())
			if err != nil {
				logger.GetLogger().Warn("Failed to get details for pipeline", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
				if progress != nil {
					progress.IncrementAPI() // エラーでも進捗をカウント
				}
				continue
			}

			detailed.AccountID = pipeline.AccountID
			detailed.AccountName = accountNameMap[pipeline.AccountID]
			detailed.SetCreated(pipeline.GetCreated())
			detailed.SetUpdated(pipeline.GetUpdated())

			detailedPipelines = append(detailedPipelines, *detailed)
			pipelinesFromAPI++

			if progress != nil {
				progress.IncrementAPI()
			}

			// 個別のパイプライン詳細をキャッシュに保存
			if err := m.cache.SetPipelineDetail(*detailed, m.config.Cache.PipelineDetailsTTL); err != nil {
				logger.GetLogger().Warn("Failed to cache pipeline detail", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
			} else {
				logger.GetLogger().Debug("Cached pipeline detail", zap.String("pipeline", pipeline.GetName()))
			}
		}

		// 進捗表示（10件ごと、または最後の件）
		if progress != nil && (i%10 == 9 || i == len(pipelines)-1) {
			fmt.Printf("\r%s", progress.GetProgressString())
		}
	}

	// 最終的な進捗表示
	if progress != nil {
		fmt.Printf("\r%s\n", progress.GetCompletionSummary())
	}

	logger.GetLogger().Debug("Pipeline details retrieval completed",
		zap.Int("total", len(detailedPipelines)),
		zap.Int("from_cache", pipelinesFromCache),
		zap.Int("from_api", pipelinesFromAPI))

	// キャッシュ使用状況を更新
	if pipelinesFromAPI == 0 && len(detailedPipelines) > 0 {
		m.cacheUsage.PipelineDetailsFromCache = true
	} else {
		m.cacheUsage.PipelineDetailsFromCache = false
	}

	return detailedPipelines, nil
}

// UpdatePipelineTrigger updates trigger settings for a specific pipeline
func (m *Manager) UpdatePipelineTrigger(ctx context.Context, pipelineName string, settings models.TriggerSettings) error {
	return m.awsClient.UpdatePipelineTrigger(ctx, pipelineName, settings)
}

// BatchUpdateTriggers updates trigger settings for multiple pipelines
func (m *Manager) BatchUpdateTriggers(ctx context.Context, targetAccounts []string, settings models.TriggerSettings, dryRun bool) error {
	pipelines, err := m.GetPipelineDetails(ctx)
	if err != nil {
		return err
	}

	targetMap := make(map[string]bool)
	for _, accountID := range targetAccounts {
		targetMap[accountID] = true
	}

	var targetPipelines []models.Pipeline
	for _, pipeline := range pipelines {
		if len(targetAccounts) == 0 || targetMap[pipeline.AccountID] {
			targetPipelines = append(targetPipelines, pipeline)
		}
	}

	fmt.Printf("Target pipelines: %d\n", len(targetPipelines))

	if dryRun {
		fmt.Println("DRY RUN - No changes will be made")
		for _, pipeline := range targetPipelines {
			fmt.Printf("Would update: %s (Account: %s)\n", pipeline.GetName(), pipeline.AccountID)
		}
		return nil
	}

	var errors []error
	for _, pipeline := range targetPipelines {
		fmt.Printf("Updating pipeline: %s\n", pipeline.GetName())
		if err := m.UpdatePipelineTrigger(ctx, pipeline.GetName(), settings); err != nil {
			errors = append(errors, fmt.Errorf("failed to update %s: %w", pipeline.GetName(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch update completed with %d errors", len(errors))
	}

	return nil
}

// ClearCache clears all cached data
func (m *Manager) ClearCache() error {
	return m.cache.ClearCache()
}

// GetCacheUsage returns the current cache usage information
func (m *Manager) GetCacheUsage() *CacheUsage {
	return m.cacheUsage
}

// hasAccountsCacheStatusSet checks if accounts cache status has been set
func (m *Manager) hasAccountsCacheStatusSet() bool {
	return m.cacheUsage.accountsStatusSet
}

// GetPipelineDetailsWithDetailedProgress retrieves detailed pipeline information with detailed progress display
func (m *Manager) GetPipelineDetailsWithDetailedProgress(ctx context.Context) ([]models.Pipeline, error) {
	progress := models.NewDetailedProgressInfo()

	// Step 1: Account情報の取得
	progress.StartAccountsRetrieval()
	accounts, err := m.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	progress.CompleteAccountsRetrieval(len(accounts), m.cacheUsage.AccountsFromCache)

	// Step 2: Pipelineリストの取得
	progress.StartPipelinesRetrieval()
	pipelines, err := m.GetPipelines(ctx)
	if err != nil {
		return nil, err
	}
	progress.CompletePipelinesRetrieval(len(pipelines), m.cacheUsage.PipelinesFromCache)

	// アカウント名のマッピングを作成
	accountNameMap := make(map[string]string)
	for _, account := range accounts {
		accountNameMap[account.ID] = account.Name
	}

	// Step 3: Pipeline詳細の取得
	progress.StartDetailsRetrieval(len(pipelines))

	var detailedPipelines []models.Pipeline
	logger.GetLogger().Debug("Checking individual pipeline caches", zap.Int("total_pipelines", len(pipelines)))

	for i, pipeline := range pipelines {
		// 個別のパイプラインキャッシュをチェック
		if cachedDetail, err := m.cache.GetPipelineDetail(pipeline.GetName()); err == nil {
			logger.GetLogger().Debug("Pipeline detail cache hit", zap.String("pipeline", pipeline.GetName()))

			// キャッシュされたパイプラインにAccountNameが設定されていない場合は設定する
			cachedPipeline := cachedDetail.Pipeline
			if cachedPipeline.AccountName == "" {
				cachedPipeline.AccountName = accountNameMap[cachedPipeline.AccountID]
			}

			detailedPipelines = append(detailedPipelines, cachedPipeline)
			progress.IncrementDetailsCache()
		} else {
			logger.GetLogger().Debug("Pipeline detail cache miss, fetching from API", zap.String("pipeline", pipeline.GetName()), zap.Error(err))

			// APIから詳細情報を取得
			detailed, err := m.awsClient.GetPipelineDetails(ctx, pipeline.GetName())
			if err != nil {
				logger.GetLogger().Warn("Failed to get details for pipeline", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
				progress.IncrementDetailsAPI() // エラーでも進捗をカウント
				continue
			}

			detailed.AccountID = pipeline.AccountID
			detailed.AccountName = accountNameMap[pipeline.AccountID]
			detailed.SetCreated(pipeline.GetCreated())
			detailed.SetUpdated(pipeline.GetUpdated())

			detailedPipelines = append(detailedPipelines, *detailed)
			progress.IncrementDetailsAPI()

			// 個別のパイプライン詳細をキャッシュに保存
			if err := m.cache.SetPipelineDetail(*detailed, m.config.Cache.PipelineDetailsTTL); err != nil {
				logger.GetLogger().Warn("Failed to cache pipeline detail", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
			} else {
				logger.GetLogger().Debug("Cached pipeline detail", zap.String("pipeline", pipeline.GetName()))
			}
		}

		// 進捗表示（10件ごと、または最後の件）
		if i%10 == 9 || i == len(pipelines)-1 {
			fmt.Printf("\r%s", progress.GetDetailsProgressString())
		}
	}

	progress.CompleteDetailsRetrieval()

	// キャッシュ使用状況を更新
	if progress.DetailsFromAPI == 0 && len(detailedPipelines) > 0 {
		m.cacheUsage.PipelineDetailsFromCache = true
	} else {
		m.cacheUsage.PipelineDetailsFromCache = false
	}

	progress.CompleteAll()
	return detailedPipelines, nil
}

// GetPipelineDetailsWithStateAndDetailedProgress retrieves detailed pipeline information including state with detailed progress display
func (m *Manager) GetPipelineDetailsWithStateAndDetailedProgress(ctx context.Context) ([]models.Pipeline, error) {
	progress := models.NewDetailedProgressInfo()

	// Step 1: Account情報の取得
	progress.StartAccountsRetrieval()
	accounts, err := m.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	progress.CompleteAccountsRetrieval(len(accounts), m.cacheUsage.AccountsFromCache)

	// Step 2: Pipelineリストの取得
	progress.StartPipelinesRetrieval()
	pipelines, err := m.GetPipelines(ctx)
	if err != nil {
		return nil, err
	}
	progress.CompletePipelinesRetrieval(len(pipelines), m.cacheUsage.PipelinesFromCache)

	// アカウント名のマッピングを作成
	accountNameMap := make(map[string]string)
	for _, account := range accounts {
		accountNameMap[account.ID] = account.Name
	}

	// Step 3: Pipeline詳細の取得
	progress.StartDetailsRetrieval(len(pipelines))

	var detailedPipelines []models.Pipeline
	logger.GetLogger().Debug("Checking individual pipeline caches", zap.Int("total_pipelines", len(pipelines)))

	for i, pipeline := range pipelines {
		// 個別のパイプラインキャッシュをチェック
		if cachedDetail, err := m.cache.GetPipelineDetail(pipeline.GetName()); err == nil {
			logger.GetLogger().Debug("Pipeline detail cache hit", zap.String("pipeline", pipeline.GetName()))

			// キャッシュされたパイプラインにAccountNameが設定されていない場合は設定する
			cachedPipeline := cachedDetail.Pipeline
			if cachedPipeline.AccountName == "" {
				cachedPipeline.AccountName = accountNameMap[cachedPipeline.AccountID]
			}

			detailedPipelines = append(detailedPipelines, cachedPipeline)
			progress.IncrementDetailsCache()
		} else {
			logger.GetLogger().Debug("Pipeline detail cache miss, fetching from API", zap.String("pipeline", pipeline.GetName()), zap.Error(err))

			// APIから詳細情報を取得
			detailed, err := m.awsClient.GetPipelineDetails(ctx, pipeline.GetName())
			if err != nil {
				logger.GetLogger().Warn("Failed to get details for pipeline", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
				progress.IncrementDetailsAPI() // エラーでも進捗をカウント
				continue
			}

			detailed.AccountID = pipeline.AccountID
			detailed.AccountName = accountNameMap[pipeline.AccountID]
			detailed.SetCreated(pipeline.GetCreated())
			detailed.SetUpdated(pipeline.GetUpdated())

			detailedPipelines = append(detailedPipelines, *detailed)
			progress.IncrementDetailsAPI()

			// 個別のパイプライン詳細をキャッシュに保存
			if err := m.cache.SetPipelineDetail(*detailed, m.config.Cache.PipelineDetailsTTL); err != nil {
				logger.GetLogger().Warn("Failed to cache pipeline detail", zap.String("pipeline", pipeline.GetName()), zap.Error(err))
			} else {
				logger.GetLogger().Debug("Cached pipeline detail", zap.String("pipeline", pipeline.GetName()))
			}
		}

		// 進捗表示（10件ごと、または最後の件）
		if i%10 == 9 || i == len(pipelines)-1 {
			fmt.Printf("\r%s", progress.GetDetailsProgressString())
		}
	}

	progress.CompleteDetailsRetrieval()

	// Step 4: Pipeline state情報の取得
	progress.StartStateRetrieval(len(detailedPipelines))

	logger.GetLogger().Debug("Adding state information to pipeline details", zap.Int("total_pipelines", len(detailedPipelines)))

	for i := range detailedPipelines {
		pipelineName := detailedPipelines[i].GetName()

		// 新しい分離されたstate情報キャッシュをチェック
		if cachedState, err := m.cache.GetPipelineState(pipelineName); err == nil {
			logger.GetLogger().Debug("Pipeline state cache hit", zap.String("pipeline", pipelineName))

			// キャッシュからstate情報を取得
			detailedPipelines[i].State = cachedState.State
			progress.IncrementStateCache()
		} else {
			logger.GetLogger().Debug("Pipeline state cache miss, fetching state from API", zap.String("pipeline", pipelineName), zap.Error(err))

			// APIからstate情報のみを取得
			stateOutput, err := m.awsClient.GetPipelineState(ctx, pipelineName)
			if err != nil {
				logger.GetLogger().Warn("Failed to get pipeline state",
					zap.String("pipeline", pipelineName),
					zap.Error(err))
				// state取得に失敗してもパイプライン自体は残す
			} else {
				// AWS SDKの型をモデルに変換
				if stateOutput.PipelineName != nil {
					state := convertAWSStateToModel(stateOutput)
					detailedPipelines[i].State = state

					// state情報のみを分離してキャッシュに保存
					if err := m.cache.SetPipelineState(pipelineName, state, m.config.Cache.PipelineDetailsTTL); err != nil {
						logger.GetLogger().Warn("Failed to cache pipeline state", zap.String("pipeline", pipelineName), zap.Error(err))
					} else {
						logger.GetLogger().Debug("Cached pipeline state", zap.String("pipeline", pipelineName))
					}
				}
			}

			progress.IncrementStateAPI()
		}

		// 進捗表示（10件ごと、または最後の件）
		if i%10 == 9 || i == len(detailedPipelines)-1 {
			fmt.Printf("\r%s", progress.GetStateProgressString())
		}
	}

	progress.CompleteStateRetrieval()

	// キャッシュ使用状況を更新
	if progress.DetailsFromAPI == 0 && len(detailedPipelines) > 0 {
		m.cacheUsage.PipelineDetailsFromCache = true
	} else {
		m.cacheUsage.PipelineDetailsFromCache = false
	}

	// Pipeline states のキャッシュ使用状況を更新
	if progress.StateFromAPI == 0 && len(detailedPipelines) > 0 {
		m.cacheUsage.PipelineStatesFromCache = true
	} else {
		m.cacheUsage.PipelineStatesFromCache = false
	}

	progress.CompleteAll()
	return detailedPipelines, nil
}

// GetPipelineDetailsWithState retrieves detailed pipeline information including state
func (m *Manager) GetPipelineDetailsWithState(ctx context.Context) ([]models.Pipeline, error) {
	return m.GetPipelineDetailsWithStateAndProgress(ctx, true)
}

// GetPipelineDetailsWithStateAndProgress retrieves detailed pipeline information including state with optional progress display
func (m *Manager) GetPipelineDetailsWithStateAndProgress(ctx context.Context, showProgress bool) ([]models.Pipeline, error) {
	logger.GetLogger().Debug("GetPipelineDetailsWithState called")

	// まず通常のパイプライン詳細を取得（キャッシュを活用）
	detailedPipelines, err := m.GetPipelineDetailsWithProgress(ctx, false)
	if err != nil {
		return nil, err
	}

	// 進捗情報を初期化
	var progress *models.ProgressInfo
	if showProgress && len(detailedPipelines) > 0 {
		progress = models.NewProgressInfo(len(detailedPipelines))
		fmt.Printf("Retrieving pipeline state information for %d pipelines...\n", len(detailedPipelines))
	}

	var pipelinesFromCache int
	var pipelinesFromAPI int

	logger.GetLogger().Debug("Adding state information to pipeline details", zap.Int("total_pipelines", len(detailedPipelines)))

	for i := range detailedPipelines {
		pipelineName := detailedPipelines[i].GetName()

		// 新しい分離されたstate情報キャッシュをチェック
		if cachedState, err := m.cache.GetPipelineState(pipelineName); err == nil {
			logger.GetLogger().Debug("Pipeline state cache hit", zap.String("pipeline", pipelineName))

			// キャッシュからstate情報を取得
			detailedPipelines[i].State = cachedState.State
			pipelinesFromCache++

			if progress != nil {
				progress.IncrementCache()
			}
		} else {
			logger.GetLogger().Debug("Pipeline state cache miss, fetching state from API", zap.String("pipeline", pipelineName), zap.Error(err))

			// APIからstate情報のみを取得
			stateOutput, err := m.awsClient.GetPipelineState(ctx, pipelineName)
			if err != nil {
				logger.GetLogger().Warn("Failed to get pipeline state",
					zap.String("pipeline", pipelineName),
					zap.Error(err))
				// state取得に失敗してもパイプライン自体は残す
			} else {
				// AWS SDKの型をモデルに変換
				if stateOutput.PipelineName != nil {
					state := convertAWSStateToModel(stateOutput)
					detailedPipelines[i].State = state

					// state情報のみを分離してキャッシュに保存
					if err := m.cache.SetPipelineState(pipelineName, state, m.config.Cache.PipelineDetailsTTL); err != nil {
						logger.GetLogger().Warn("Failed to cache pipeline state", zap.String("pipeline", pipelineName), zap.Error(err))
					} else {
						logger.GetLogger().Debug("Cached pipeline state", zap.String("pipeline", pipelineName))
					}
				}
			}

			pipelinesFromAPI++

			if progress != nil {
				progress.IncrementAPI()
			}
		}

		// 進捗表示（10件ごと、または最後の件）
		if progress != nil && (i%10 == 9 || i == len(detailedPipelines)-1) {
			fmt.Printf("\r%s", progress.GetProgressString())
		}
	}

	// 最終的な進捗表示
	if progress != nil {
		fmt.Printf("\r%s\n", progress.GetCompletionSummary())
	}

	logger.GetLogger().Debug("Pipeline details with state retrieval completed",
		zap.Int("total", len(detailedPipelines)),
		zap.Int("from_cache", pipelinesFromCache),
		zap.Int("from_api", pipelinesFromAPI))

	// Pipeline states のキャッシュ使用状況を更新
	if pipelinesFromAPI == 0 && len(detailedPipelines) > 0 {
		m.cacheUsage.PipelineStatesFromCache = true
	} else {
		m.cacheUsage.PipelineStatesFromCache = false
	}

	return detailedPipelines, nil
}

// convertAWSStateToModel converts AWS SDK pipeline state to our model
func convertAWSStateToModel(stateOutput *codepipeline.GetPipelineStateOutput) *models.PipelineState {
	if stateOutput.PipelineName == nil {
		return nil
	}

	state := &models.PipelineState{
		PipelineName: *stateOutput.PipelineName,
	}

	if stateOutput.PipelineVersion != nil {
		state.PipelineVersion = *stateOutput.PipelineVersion
	}

	if stateOutput.Created != nil {
		state.Created = stateOutput.Created
	}

	if stateOutput.Updated != nil {
		state.Updated = stateOutput.Updated
	}

	// StageStatesを変換
	for _, stageState := range stateOutput.StageStates {
		modelStageState := models.StageState{}

		if stageState.StageName != nil {
			modelStageState.StageName = *stageState.StageName
		}

		// InboundTransitionStateを変換
		if stageState.InboundTransitionState != nil {
			modelStageState.InboundTransitionState = &models.TransitionState{
				Enabled: stageState.InboundTransitionState.Enabled,
			}

			if stageState.InboundTransitionState.LastChangedBy != nil {
				modelStageState.InboundTransitionState.LastChangedBy = *stageState.InboundTransitionState.LastChangedBy
			}

			if stageState.InboundTransitionState.LastChangedAt != nil {
				modelStageState.InboundTransitionState.LastChangedAt = stageState.InboundTransitionState.LastChangedAt
			}

			if stageState.InboundTransitionState.DisabledReason != nil {
				modelStageState.InboundTransitionState.DisabledReason = *stageState.InboundTransitionState.DisabledReason
			}
		}

		// ActionStatesを変換
		for _, actionState := range stageState.ActionStates {
			modelActionState := models.ActionState{}

			if actionState.ActionName != nil {
				modelActionState.ActionName = *actionState.ActionName
			}

			if actionState.EntityUrl != nil {
				modelActionState.EntityUrl = *actionState.EntityUrl
			}

			if actionState.RevisionUrl != nil {
				modelActionState.RevisionUrl = *actionState.RevisionUrl
			}

			// CurrentRevisionを変換
			if actionState.CurrentRevision != nil {
				modelActionState.CurrentRevision = &models.ActionRevision{}

				if actionState.CurrentRevision.RevisionId != nil {
					modelActionState.CurrentRevision.RevisionId = *actionState.CurrentRevision.RevisionId
				}

				if actionState.CurrentRevision.RevisionChangeId != nil {
					modelActionState.CurrentRevision.RevisionChangeId = *actionState.CurrentRevision.RevisionChangeId
				}

				if actionState.CurrentRevision.Created != nil {
					modelActionState.CurrentRevision.Created = *actionState.CurrentRevision.Created
				}
			}

			// LatestExecutionを変換
			if actionState.LatestExecution != nil {
				modelActionState.LatestExecution = &models.ActionExecution{}

				if actionState.LatestExecution.ActionExecutionId != nil {
					modelActionState.LatestExecution.ActionExecutionId = *actionState.LatestExecution.ActionExecutionId
				}

				if actionState.LatestExecution.Status != "" {
					modelActionState.LatestExecution.Status = string(actionState.LatestExecution.Status)
				}

				if actionState.LatestExecution.Summary != nil {
					modelActionState.LatestExecution.Summary = *actionState.LatestExecution.Summary
				}

				if actionState.LatestExecution.LastStatusChange != nil {
					modelActionState.LatestExecution.LastStatusChange = actionState.LatestExecution.LastStatusChange
				}

				if actionState.LatestExecution.Token != nil {
					modelActionState.LatestExecution.Token = *actionState.LatestExecution.Token
				}

				if actionState.LatestExecution.LastUpdatedBy != nil {
					modelActionState.LatestExecution.LastUpdatedBy = *actionState.LatestExecution.LastUpdatedBy
				}

				if actionState.LatestExecution.ExternalExecutionId != nil {
					modelActionState.LatestExecution.ExternalExecutionId = *actionState.LatestExecution.ExternalExecutionId
				}

				if actionState.LatestExecution.ExternalExecutionUrl != nil {
					modelActionState.LatestExecution.ExternalExecutionUrl = *actionState.LatestExecution.ExternalExecutionUrl
				}

				if actionState.LatestExecution.PercentComplete != nil {
					modelActionState.LatestExecution.PercentComplete = *actionState.LatestExecution.PercentComplete
				}

				// ErrorDetailsを変換
				if actionState.LatestExecution.ErrorDetails != nil {
					modelActionState.LatestExecution.ErrorDetails = &models.ErrorDetails{}

					if actionState.LatestExecution.ErrorDetails.Code != nil {
						modelActionState.LatestExecution.ErrorDetails.Code = *actionState.LatestExecution.ErrorDetails.Code
					}

					if actionState.LatestExecution.ErrorDetails.Message != nil {
						modelActionState.LatestExecution.ErrorDetails.Message = *actionState.LatestExecution.ErrorDetails.Message
					}
				}
			}

			modelStageState.ActionStates = append(modelStageState.ActionStates, modelActionState)
		}

		// LatestExecutionを変換
		if stageState.LatestExecution != nil {
			modelStageState.LatestExecution = &models.StageExecution{}

			if stageState.LatestExecution.PipelineExecutionId != nil {
				modelStageState.LatestExecution.PipelineExecutionId = *stageState.LatestExecution.PipelineExecutionId
			}

			if stageState.LatestExecution.Status != "" {
				modelStageState.LatestExecution.Status = string(stageState.LatestExecution.Status)
			}
		}

		state.StageStates = append(state.StageStates, modelStageState)
	}

	return state
}
