# AFT CodePipeline 操作ツールの仕様書

## 概要

このツールは、AWS Control Tower Account Factory for Terraform (AFT) の CodePipeline を CLI から操作するためのツールの仕様書です。

## 目的

- AFT管理下の各アカウントに対応したCodePipelineの設定の確認や編集を個別、または一括で行う
- AFT管理下の各アカウントに対応したCodePipelineの実行状況の確認
- AFT管理下の各アカウントに対応したCodePipelineの実行を個別、または一括で行う
- 設定変更の自動化とミスの削減

## 技術スタック

- **言語**: Go 1.23+ (最小要件: Go 1.21)
- **AWS SDK**: AWS SDK for Go v2
- **CLI Framework**: cobra
- **設定管理**: viper
- **ログ**: uber-go/zap
- **テスト**: testify

## アーキテクチャ

### プロジェクト構造
ツールは以下のモジュール構成で設計されています：

- **cmd/**: CLIコマンドの定義（list, update, batch-update, execute, fix-triggers, fix-connections, fix-pipeline-type, test-triggers, test-connections, cache, version）
- **internal/aws/**: AWS API操作（Organizations, CodePipeline）
- **internal/cache/**: データキャッシュ機能
- **internal/config/**: 設定管理
- **internal/models/**: データモデル定義
- **internal/utils/**: ユーティリティ機能（フォーマッター、カラー出力）
- **internal/validation/**: バリデーション機能
- **internal/pipeline/**: パイプライン処理機能
- **internal/logger/**: ログ機能
- **internal/errors/**: エラーハンドリング
- **pkg/aft/**: AFT操作の中核機能
- **configs/**: 設定ファイル

### データモデル

#### アカウント情報
- アカウントID、名前、メールアドレス、ステータスを管理
- キャッシュ機能によりAPIコール回数を削減

#### パイプライン情報
- パイプライン名、バージョン、作成・更新日時
- ソース設定（プロバイダー、リポジトリ、ブランチ、トリガータイプ）
- トリガータイプ（Webhook、Polling、None）の分類

#### 設定管理
- キャッシュ設定（ディレクトリ、TTL）
- 除外アカウント設定
- AWS設定（リージョン、プロファイル）
- 出力フォーマット設定
- バッチ更新設定

## キャッシュ機能詳細仕様

### キャッシュデータタイプ

#### 1. Account情報キャッシュ
**対応API**: AWS Organizations `ListAccounts`
**目的**: pipelineとaccountの紐づけを行うためのデータ
**データ構造**:
```go
type Account struct {
    ID     string `json:"id"`     // アカウントID
    Name   string `json:"name"`   // アカウント名
    Email  string `json:"email"`  // アカウントのメールアドレス
    Status string `json:"status"` // アカウントのステータス（ACTIVE等）
}

type AccountCache struct {
    Accounts []Account `json:"accounts"` // アカウント一覧
    CachedAt time.Time `json:"cached_at"` // キャッシュ作成時刻
    TTL      int       `json:"ttl"`       // キャッシュ有効期間（秒）
}
```

**利用用途**:
- Pipeline名にaccount idが含まれているため、pipelineがどのアカウントと対応しているのかを人間がわかるように、アカウント名との対応付けに利用
- アカウント一覧表示での名前解決
- 除外アカウント設定での名前表示

**キャッシュファイル**: `~/.aft-pipeline-tool/cache/accounts.json`
**デフォルトTTL**: 3600秒（1時間）

#### 2. Pipelineリスト情報キャッシュ
**対応API**: AWS CodePipeline `ListPipelines`
**目的**: pipeline名とaccount名の紐づけ、pipelineの簡易的な情報一覧表示
**データ構造**:
```go
type PipelineCache struct {
    Pipelines []Pipeline `json:"pipelines"` // パイプライン一覧
    CachedAt  time.Time  `json:"cached_at"` // キャッシュ作成時刻
    TTL       int        `json:"ttl"`       // キャッシュ有効期間（秒）
}

// Pipeline構造体の簡易情報部分
type Pipeline struct {
    Pipeline PipelineDeclaration `json:"pipeline"` // パイプライン宣言
    Metadata PipelineMetadata    `json:"metadata"` // メタデータ
    AccountID   string `json:"account_id"`   // 関連アカウントID
    AccountName string `json:"account_name"` // 関連アカウント名
}
```

**利用用途**:
- Pipeline一覧表示（`list`コマンド）
- Pipeline名とアカウント名の紐づけ表示
- AFTパイプラインの識別とフィルタリング
- バッチ処理での対象パイプライン選択

**キャッシュファイル**: `~/.aft-pipeline-tool/cache/pipelines.json`
**デフォルトTTL**: 1800秒（30分）

#### 3. Pipeline詳細情報キャッシュ
**対応API**: AWS CodePipeline `GetPipeline`
**目的**: connectionの変更、triggerの変更などの設定変更処理
**データ構造**:
```go
type PipelineDetailCache struct {
    Pipeline Pipeline  `json:"pipeline"`  // 完全なパイプライン詳細情報
    CachedAt time.Time `json:"cached_at"` // キャッシュ作成時刻
    TTL      int       `json:"ttl"`       // キャッシュ有効期間（秒）
}

// 詳細なパイプライン宣言
type PipelineDeclaration struct {
    Name          string        `json:"name"`          // パイプライン名
    RoleArn       string        `json:"roleArn"`       // 実行ロールARN
    ArtifactStore ArtifactStore `json:"artifactStore"` // アーティファクトストア設定
    Stages        []Stage       `json:"stages"`        // ステージ定義
    Version       int32         `json:"version"`       // パイプラインバージョン
    ExecutionMode string        `json:"executionMode"` // 実行モード
    PipelineType  string        `json:"pipelineType"`  // パイプラインタイプ（V1/V2）
    Triggers      []Trigger     `json:"triggers"`      // トリガー設定（V2形式）
}
```

**利用用途**:
- Connection ARNの更新（`fix-connections`コマンド）
- Trigger設定の変更（`fix-triggers`コマンド）
- Pipeline設定の詳細表示
- Pipeline設定の更新処理
- JSON形式でのconnectionやtrigger設定の操作

**キャッシュファイル**: `~/.aft-pipeline-tool/cache/pipeline_details/{pipeline_name}.json`
**デフォルトTTL**: 900秒（15分）

#### 4. Pipeline State情報キャッシュ
**対応API**: AWS CodePipeline `GetPipelineState`
**目的**: pipelineの実行状態確認
**データ構造**:
```go
type PipelineStateCache struct {
    State    *PipelineState `json:"state"`     // パイプライン状態情報
    CachedAt time.Time      `json:"cached_at"` // キャッシュ作成時刻
    TTL      int            `json:"ttl"`       // キャッシュ有効期間（秒）
}

type PipelineState struct {
    PipelineName    string       `json:"pipelineName"`    // パイプライン名
    PipelineVersion int32        `json:"pipelineVersion"` // パイプラインバージョン
    StageStates     []StageState `json:"stageStates"`     // ステージ状態一覧
    Created         *time.Time   `json:"created,omitempty"`  // 作成日時
    Updated         *time.Time   `json:"updated,omitempty"`  // 更新日時
}

type StageState struct {
    StageName              string           `json:"stageName"`              // ステージ名
    InboundTransitionState *TransitionState `json:"inboundTransitionState"` // 遷移状態
    ActionStates           []ActionState    `json:"actionStates"`           // アクション状態一覧
    LatestExecution        *StageExecution  `json:"latestExecution"`        // 最新実行情報
}
```

**利用用途**:
- Pipeline実行状態の確認（`execute status`コマンド）
- 実行中パイプラインの監視
- ステージ別の実行状況表示
- アクション別の実行結果確認
- エラー状態の詳細表示

**キャッシュファイル**: `~/.aft-pipeline-tool/cache/pipeline_states/{pipeline_name}.json`
**デフォルトTTL**: 300秒（5分）※実行状態は頻繁に変更されるため短めに設定

### キャッシュ管理機能

#### キャッシュインターフェース
```go
type Cache interface {
    // Account operations
    GetAccounts() (*models.AccountCache, error)
    SetAccounts(accounts []models.Account, ttl int) error

    // Pipeline operations
    GetPipelines() (*models.PipelineCache, error)
    SetPipelines(pipelines []models.Pipeline, ttl int) error

    // Pipeline detail operations
    GetPipelineDetail(pipelineName string) (*models.PipelineDetailCache, error)
    SetPipelineDetail(pipeline models.Pipeline, ttl int) error

    // Pipeline state operations
    GetPipelineState(pipelineName string) (*models.PipelineStateCache, error)
    SetPipelineState(pipelineName string, state *models.PipelineState, ttl int) error

    // Cache management
    ClearCache() error
    DeletePipelineCache(pipelineName string) error
}
```

#### キャッシュ有効期限管理
- 各キャッシュアイテムは個別のTTL（Time To Live）を持つ
- キャッシュ取得時に自動的に有効期限をチェック
- 期限切れの場合は自動的にAWS APIから最新データを取得
- 手動でのキャッシュクリア機能（`cache clear`コマンド）

#### キャッシュファイル構造
```
~/.aft-pipeline-tool/cache/
├── accounts.json                    # アカウント情報キャッシュ
├── pipelines.json                   # パイプライン一覧キャッシュ
├── pipeline_details/                # パイプライン詳細キャッシュディレクトリ
│   ├── {pipeline_name_1}.json
│   ├── {pipeline_name_2}.json
│   └── ...
├── pipeline_states/                 # パイプライン状態キャッシュディレクトリ
│   ├── {pipeline_name_1}.json
│   ├── {pipeline_name_2}.json
│   └── ...
└── pipeline_executions/             # パイプライン実行履歴キャッシュディレクトリ
    ├── {pipeline_name_1}.json
    ├── {pipeline_name_2}.json
    └── ...
```

#### キャッシュ設定
```yaml
cache:
  directory: ~/.aft-pipeline-tool/cache  # キャッシュディレクトリ
  accounts_ttl: 3600                     # アカウント情報TTL（秒）
  pipelines_ttl: 1800                    # パイプライン一覧TTL（秒）
  pipeline_details_ttl: 900              # パイプライン詳細TTL（秒）
  pipeline_states_ttl: 300               # パイプライン状態TTL（秒）
  pipeline_executions_ttl: 600           # パイプライン実行履歴TTL（秒）
```

### キャッシュ利用パターン

#### 1. 初回データ取得パターン
1. キャッシュファイルの存在確認
2. 存在しない場合：AWS APIから取得 → キャッシュに保存
3. 存在する場合：有効期限チェック → 期限切れならAWS APIから取得

#### 2. データ更新パターン
1. 設定変更後は該当するキャッシュを無効化
2. 次回アクセス時に最新データを取得
3. バッチ更新時は関連するすべてのキャッシュを無効化

#### 3. パフォーマンス最適化パターン
- 頻繁にアクセスされるアカウント情報は長めのTTL
- 変更頻度の高いパイプライン状態は短めのTTL
- 詳細情報は中程度のTTLで設定変更時のみ無効化

## 主要機能

### 1. AWS クライアント管理
- AWS SDK v2を使用したクライアント初期化
- リージョンとプロファイルの設定対応
- Organizations、CodePipelineサービスへの接続管理

### 2. Organizations API連携
- アカウント一覧の取得とページネーション対応
- アクティブアカウントのフィルタリング
- 除外アカウント設定による絞り込み機能

### 3. CodePipeline API連携
- AFTパイプラインの自動識別（命名パターンによる判定）
- パイプライン詳細情報の取得
- ソースアクションの設定解析
- トリガー設定の更新機能

### 4. キャッシュ機能
- ファイルベースキャッシュによるパフォーマンス向上
- TTL（Time To Live）による自動キャッシュ無効化
- アカウント情報とパイプライン情報の個別キャッシュ管理

### 5. AFTマネージャー
- キャッシュとAWS APIの統合管理
- アカウント・パイプライン情報の統一的な取得
- バッチ更新処理の制御
- エラーハンドリングと警告表示

### 6. CLIコマンド機能

#### listコマンド
**目的**: pipelineのリスト情報とaccount名をマージした情報の表示

**機能**:
- AFTパイプライン一覧の表示
- パイプライン名とアカウント名の紐づけ表示
- 複数出力フォーマット対応（table, json, csv）
- アカウントIDによるフィルタリング機能
- キャッシュされたstate情報がある場合は追加表示
- LATEST STAGE UPDATEの新しい順でのソート機能

**使用例**:
```bash
# 基本的な一覧表示
aft-pipeline-tool list

# 詳細情報付きで表示
aft-pipeline-tool list --show-details

# 特定アカウントでフィルタリング
aft-pipeline-tool list --account-filter "123456789012"

# JSON形式で出力
aft-pipeline-tool list --format json
```

**表示内容**:
- パイプライン名
- 関連アカウント名
- パイプラインタイプ（V1/V2）
- トリガー設定
- 最終更新日時
- 実行状態（キャッシュされている場合）

**表示モード**:
- **シンプル表示モード**（デフォルト）: アカウント名とパイプライン名のみ表示
- **詳細表示モード**（--show-detailsフラグ）: すべての項目を表示
- **state情報表示**: キャッシュされたstate情報がある場合は実行状態とLATEST STAGE UPDATEを追加表示

#### updateコマンド
- 個別パイプラインのトリガー設定更新
- ブランチ設定の変更
- トリガータイプの切り替え（Webhook/Polling/None）

#### batch-updateコマンド
- 複数パイプラインの一括更新
- 対象アカウントの指定機能
- ドライラン機能による事前確認

#### statusコマンド
**目的**: pipeline一覧とpipeline個々のstate情報をマージした情報の表示

**機能**:
- AFTパイプライン一覧の表示
- パイプライン実行状態情報を常に含めて表示
- 複数出力フォーマット対応（table, json, csv）
- アカウントIDによるフィルタリング機能
- LATEST STAGE UPDATEの新しい順でのソート
- 個別パイプラインの詳細状態確認
- 全パイプラインの状態一覧表示

**使用例**:
```bash
# 全パイプライン一覧と実行状態を表示
aft-pipeline-tool status

# 詳細情報付きで表示
aft-pipeline-tool status --show-details

# 特定アカウントでフィルタリング
aft-pipeline-tool status --account-filter "123456789012"

# 個別パイプラインの詳細状態確認
aft-pipeline-tool status pipeline-name

# 詳細情報付きで個別パイプライン状態確認
aft-pipeline-tool status pipeline-name --details
```

**表示内容**:
- パイプライン名
- 関連アカウント名
- パイプラインタイプ（V1/V2）
- トリガー設定
- 最終更新日時
- 実行状態（常に取得・表示）
- ステージ別実行状況
- 最新実行ID
- 実行開始・終了時刻

**listコマンドとの違い**:
- **statusコマンド**: パイプライン実行状態情報を常に取得・表示（AWS APIを呼び出し）
- **listコマンド**: キャッシュされたstate情報がある場合のみ表示（APIコールなし）

**表示モード**:
- **全パイプライン一覧モード**（引数なし）: 全AFTパイプラインの状態一覧を表示
- **個別パイプライン詳細モード**（パイプライン名指定）: 特定パイプラインの詳細状態を表示

#### executeコマンド
**目的**: パイプライン実行の開始・停止・監視機能

**デフォルト動作**:
- 引数なしで実行した場合、`execute list`と同じ動作（パイプライン一覧と実行状態を表示）
- 実行状態情報を常に含めて表示

**サブコマンド**:

##### execute list
**機能**: パイプライン一覧と実行状態の表示
- AFTパイプライン一覧の表示
- パイプライン実行状態情報を常に含めて表示
- 複数出力フォーマット対応（table, json, csv）
- アカウントIDによるフィルタリング機能
- LATEST STAGE UPDATEの新しい順でのソート

**使用例**:
```bash
# パイプライン一覧と実行状態を表示
aft-pipeline-tool execute list

# 詳細情報付きで表示
aft-pipeline-tool execute list --show-details

# 特定アカウントでフィルタリング
aft-pipeline-tool execute list --account-filter "123456789012"
```

##### execute start
**機能**: パイプライン実行の開始
- 個別パイプラインの実行開始
- 全AFTパイプラインの一括実行開始（--all）
- 実行完了待機機能（--wait）
- 強制実行機能（--force）

**使用例**:
```bash
# 個別パイプライン実行
aft-pipeline-tool execute start pipeline-name

# 全パイプライン実行
aft-pipeline-tool execute start --all

# 実行完了まで待機
aft-pipeline-tool execute start pipeline-name --wait
```

##### execute start-from-file
**機能**: ファイルからの一括実行
- ファイルに記載されたパイプライン名の一括実行
- 並行実行とシーケンシャル実行の選択
- 実行完了待機機能（--wait）
- 強制実行機能（--force）

**使用例**:
```bash
# ファイルからパイプライン実行
aft-pipeline-tool execute start-from-file pipelines.txt

# シーケンシャル実行
aft-pipeline-tool execute start-from-file pipelines.txt --sequential
```

##### execute stop
**機能**: パイプライン実行の停止
- 実行中パイプラインの停止
- 特定実行IDの指定停止

**使用例**:
```bash
# 最新の実行中パイプラインを停止
aft-pipeline-tool execute stop pipeline-name

# 特定実行IDを停止
aft-pipeline-tool execute stop pipeline-name execution-id
```

##### execute history
**機能**: 実行履歴の表示
- パイプライン実行履歴の表示
- 表示件数の制限
- 詳細情報表示オプション

**使用例**:
```bash
# 実行履歴表示（デフォルト10件）
aft-pipeline-tool execute history pipeline-name

# 表示件数を指定
aft-pipeline-tool execute history pipeline-name --max-results 20

# 詳細情報付きで表示
aft-pipeline-tool execute history pipeline-name --details
```

#### fix-triggersコマンド
- パイプライントリガーの自動修正機能
- 単一ソースアクションのみトリガー設定
- 全トリガーの削除機能
- ドライラン機能による事前確認

#### fix-connectionsコマンド
- ConnectionArnの一括更新機能
- 特定ソースアクションのConnectionArn更新
- 全パイプラインまたは個別パイプラインの対象選択
- ドライラン機能による事前確認

#### fix-pipeline-typeコマンド
- パイプラインタイプ（V1/V2）の変更機能
- 全パイプラインまたは個別パイプラインの対象選択
- ドライラン機能による事前確認

#### test-triggersコマンド
- トリガー機能のテスト
- パイプライン設定ファイルを使用したテスト実行

#### test-connectionsコマンド
- ConnectionArn更新機能のテスト
- パイプライン設定ファイルを使用したテスト実行

#### cacheコマンド
- キャッシュ管理機能
- キャッシュのクリア・更新・確認

#### versionコマンド
- ツールのバージョン情報表示

### 7. 出力フォーマット機能
- テーブル形式：人間が読みやすい表形式
- JSON形式：プログラムでの処理に適した構造化データ
- CSV形式：スプレッドシートでの分析用

## 設定仕様

### 設定ファイル構造

#### メイン設定（config.yaml）
```yaml
cache:
  directory: ~/.aft-pipeline-tool/cache
  accounts_ttl: 3600        # アカウント情報キャッシュTTL（秒）
  pipelines_ttl: 1800       # パイプライン情報キャッシュTTL（秒）
  pipeline_details_ttl: 900 # パイプライン詳細キャッシュTTL（秒）

exclude_accounts:
  - "111111111111"  # 除外するアカウントID
  - "222222222222"
  - "333333333333"

aws:
  region: us-east-1    # AWSリージョン（オプション）
  profile: default     # AWSプロファイル（オプション）

output:
  format: table        # 出力フォーマット（table, json, csv）
  color: true          # カラー出力の有効/無効

batch_update:
  dry_run: true              # デフォルトでドライラン実行
  target_accounts: []        # 対象アカウントの制限
  settings:
    branch: main             # デフォルトブランチ
    trigger_enabled: true    # トリガー有効化
    polling_enabled: false   # ポーリング有効化

execution:
  max_concurrent: 3    # 最大同時実行数
  max_pipelines: 50    # ファイルから読み込む最大パイプライン数
  timeout: 3600        # 実行完了待機のタイムアウト（秒）
  sequential: false    # シーケンシャル実行モード
```

### 設定項目詳細

#### キャッシュ設定
- **directory**: キャッシュファイルの保存ディレクトリ
- **accounts_ttl**: アカウント情報のキャッシュ有効期間（秒）
- **pipelines_ttl**: パイプライン一覧のキャッシュ有効期間（秒）
- **pipeline_details_ttl**: パイプライン詳細のキャッシュ有効期間（秒）

#### AWS設定
- **region**: 使用するAWSリージョン（未指定時は環境設定を使用）
- **profile**: 使用するAWSプロファイル（未指定時はdefaultまたは環境設定を使用）

#### 出力設定
- **format**: デフォルト出力フォーマット（table, json, csv）
- **color**: カラー出力の有効/無効

#### バッチ更新設定
- **dry_run**: デフォルトでドライラン実行するかどうか
- **target_accounts**: 更新対象とするアカウントIDのリスト（空の場合は全アカウント）
- **settings**: バッチ更新時のデフォルト設定

#### 実行設定
- **max_concurrent**: 並行実行時の最大同時実行数
- **max_pipelines**: ファイルから読み込む最大パイプライン数
- **timeout**: 実行完了待機のタイムアウト時間（秒）
- **sequential**: シーケンシャル実行モードの有効/無効

## エラーハンドリング

### 一般的なエラー対応
- AWS認証エラー：プロファイル設定の確認を促す
- 権限不足エラー：必要なIAM権限の表示
- ネットワークエラー：リトライ機能による自動復旧
- キャッシュエラー：警告表示のみでAPIから直接取得

### バッチ処理エラー対応
- 部分的な失敗を許容し、成功した更新は継続
- 失敗したパイプラインのリストを表示
- エラー詳細の個別表示

## セキュリティ考慮事項

### 認証・認可
- AWS IAMロールベースの認証
- 最小権限の原則に基づく権限設定
- MFA（多要素認証）の推奨

### 必要なIAM権限
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "organizations:ListAccounts",
        "codepipeline:ListPipelines",
        "codepipeline:GetPipeline",
        "codepipeline:UpdatePipeline",
        "codepipeline:StartPipelineExecution",
        "codepipeline:StopPipelineExecution",
        "codepipeline:GetPipelineExecution",
        "codepipeline:ListPipelineExecutions"
      ],
      "Resource": "*"
    }
  ]
}
```

**権限の説明**:
- `organizations:ListAccounts`: AFT管理下のアカウント一覧取得
- `codepipeline:ListPipelines`: パイプライン一覧取得
- `codepipeline:GetPipeline`: パイプライン詳細情報取得
- `codepipeline:UpdatePipeline`: パイプライン設定更新
- `codepipeline:StartPipelineExecution`: パイプライン実行開始
- `codepipeline:StopPipelineExecution`: パイプライン実行停止
- `codepipeline:GetPipelineExecution`: パイプライン実行詳細取得
- `codepipeline:ListPipelineExecutions`: パイプライン実行履歴取得

### データ保護
- キャッシュファイルの適切な権限設定（600）
- 機密情報のログ出力回避
- 設定ファイルの暗号化推奨

## パフォーマンス最適化

### キャッシュ戦略
- アカウント情報：1時間キャッシュ（変更頻度が低い）
- パイプライン情報：30分キャッシュ（変更頻度が中程度）
- 手動キャッシュクリア機能

### API呼び出し最適化
- ページネーション対応による大量データ処理
- 並行処理による処理時間短縮
- レート制限対応

## 拡張性

### 将来的な機能拡張
- 他のAWSサービスとの連携（CloudWatch、SNS）
- Webhook設定の自動化
- 設定テンプレート機能
- Web UIの提供

### プラグイン機能
- カスタムフォーマッターの追加
- 外部システムとの連携
- 独自バリデーションルールの実装

## テスト戦略

### 単体テスト
- 各モジュールの独立したテスト
- モックを使用したAWS API呼び出しのテスト
- エラーケースの網羅的なテスト

### 統合テスト
- 実際のAWS環境を使用したエンドツーエンドテスト
- キャッシュ機能の動作確認
- 複数アカウント環境でのテスト

### パフォーマンステスト
- 大量アカウント環境での動作確認
- メモリ使用量の監視
- 処理時間の測定

## 運用・保守

### ログ機能
- 構造化ログによる運用性向上
- ログレベルの動的変更
- ローテーション機能

### 監視・アラート
- 処理失敗時の通知機能
- パフォーマンス指標の収集
- ヘルスチェック機能

### バックアップ・復旧
- 設定変更前のバックアップ自動作成
- ロールバック機能
- 設定履歴の管理
