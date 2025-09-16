# AFT CodePipeline の操作をするツールの仕様書

## 概要

AWS Control Tower Account Factory for Terraform (AFT) のCodePipelineの操作を行うツールの仕様書です。

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
- AFTパイプライン一覧の表示
- 複数出力フォーマット対応（table, json, csv）
- アカウントIDによるフィルタリング機能

#### updateコマンド
- 個別パイプラインのトリガー設定更新
- ブランチ設定の変更
- トリガータイプの切り替え（Webhook/Polling/None）

#### batch-updateコマンド
- 複数パイプラインの一括更新
- 対象アカウントの指定機能
- ドライラン機能による事前確認

#### executeコマンド
- パイプライン実行の開始・停止・監視機能
- 個別パイプラインの実行開始（start）
- 全AFTパイプラインの一括実行開始（start --all）
- ファイルからの一括実行（start-from-file）
- パイプライン実行の停止（stop）
- 実行ステータスの確認（status）
- 実行履歴の表示（history）
- 並行実行とシーケンシャル実行の選択
- 実行完了待機機能（--wait）
- 強制実行機能（--force）

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
