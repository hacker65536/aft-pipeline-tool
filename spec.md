# AFT CodePipeline Git Triggers 編集ツール仕様書

## 概要

AWS Control Tower Account Factory for Terraform (AFT) のCodePipelineにおけるGitトリガー設定を管理するGoツールの仕様書です。

## 目的

- AFT管理下のアカウントのCodePipelineのGitトリガー設定を一括で確認・編集
- 複数アカウントのパイプライン設定を効率的に管理
- 設定変更の自動化とミスの削減

## 技術スタック

- **言語**: Go 1.21+
- **AWS SDK**: AWS SDK for Go v2
- **CLI Framework**: cobra
- **設定管理**: viper
- **ログ**: uber-go/zap
- **テスト**: testify

## アーキテクチャ

### プロジェクト構造
ツールは以下のモジュール構成で設計されています：

- **cmd/**: CLIコマンドの定義（list, update, batch-update, diff）
- **internal/aws/**: AWS API操作（Organizations, CodePipeline）
- **internal/cache/**: データキャッシュ機能
- **internal/config/**: 設定管理
- **internal/models/**: データモデル定義
- **internal/utils/**: ユーティリティ機能（フォーマッター、バリデーション）
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

#### diffコマンド
- 設定変更前後の差分表示
- 変更内容の可視化

### 7. 出力フォーマット機能
- テーブル形式：人間が読みやすい表形式
- JSON形式：プログラムでの処理に適した構造化データ
- CSV形式：スプレッドシートでの分析用

## 使用方法

### 基本コマンド

```bash
# パイプライン一覧表示
aft-pipeline-tool list

# JSON形式で出力
aft-pipeline-tool list --format json

# 特定アカウントでフィルタリング
aft-pipeline-tool list --account-filter "123456789012"

# 個別パイプライン更新
aft-pipeline-tool update --pipeline "123456789012-customizations-pipeline" --branch "main" --trigger webhook

# バッチ更新（ドライラン）
aft-pipeline-tool batch-update --dry-run --branch "main" --trigger polling

# 設定差分表示
aft-pipeline-tool diff --pipeline "123456789012-customizations-pipeline" --branch "develop"
```

### 設定ファイル

#### メイン設定（config.yaml）
```yaml
cache:
  directory: ~/.aft-pipeline-tool/cache
  accounts_ttl: 3600
  pipelines_ttl: 1800

aws:
  region: us-east-1
  profile: default

output:
  format: table
  color: true

batch_update:
  dry_run: true
  settings:
    branch: main
    trigger_enabled: true
    polling_enabled: false
```

#### 除外アカウント設定（exclude_accounts.yaml）
```yaml
exclude_accounts:
  - "111111111111"  # 管理アカウント
  - "222222222222"  # ログアーカイブアカウント
  - "333333333333"  # 監査アカウント
```

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
        "codepipeline:UpdatePipeline"
      ],
      "Resource": "*"
    }
  ]
}
```

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
