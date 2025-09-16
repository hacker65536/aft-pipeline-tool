# AFT CodePipeline Git Triggers 編集ツール

AWS Control Tower Account Factory for Terraform (AFT) のCodePipelineにおけるGitトリガー設定を管理するGoツールです。

## 概要

このツールは、AFT管理下の複数アカウントのCodePipelineを効率的に管理するためのCLIツールです。パイプラインの設定確認・編集から実行管理まで、AFT運用に必要な操作を一元的に行うことができます。

### 主な特徴

- **一括操作**: 複数アカウントのパイプラインを同時に管理
- **安全な更新**: ドライラン機能による事前確認
- **柔軟な出力**: テーブル、JSON、CSV形式での結果表示
- **高速処理**: キャッシュ機能によるパフォーマンス向上
- **実行管理**: パイプラインの開始・停止・監視機能

### 関連ドキュメント

- **[技術仕様書](docs/spec.md)**: アーキテクチャ、データモデル、セキュリティ考慮事項などの詳細仕様
- **[GitHub Workflows](docs/github-workflows.md)**: CI/CDワークフローの設定
- **[ワークフロー最適化](docs/workflow-optimization.md)**: パフォーマンス最適化に関する情報

## AI開発について

このツールは主にAI（Claude）によって開発されました。要件定義から実装、テスト、ドキュメント作成まで、開発プロセスの大部分がAIによって自動化されています。これにより、短期間での高品質なツール開発を実現しています。

## 機能

- AFTパイプライン一覧の表示
- 個別パイプラインのトリガー設定更新
- 複数パイプラインの一括更新
- パイプライントリガーの自動修正（fix-triggers）
- パイプラインConnectionArnの一括更新（fix-connections）
- パイプラインタイプの変更（fix-pipeline-type）
- **パイプライン実行管理（execute）**
- 複数出力フォーマット対応（table, json, csv）
- キャッシュ機能によるパフォーマンス向上
- ドライラン機能

## インストール

### 前提条件

- Go 1.21以上
- AWS CLI設定済み
- 適切なIAM権限

### ビルド

```bash
go build -o aft-pipeline-tool
```

## 使用方法

### 基本コマンド

#### パイプライン一覧表示

```bash
# テーブル形式で表示
./aft-pipeline-tool list

# JSON形式で表示
./aft-pipeline-tool list --format json

# 特定アカウントでフィルタリング
./aft-pipeline-tool list --account-filter "123456789012"
```

#### 個別パイプライン更新

```bash
# ブランチをmainに変更
./aft-pipeline-tool update --pipeline "123456789012-customizations-pipeline" --branch "main"

# トリガーをwebhookに設定
./aft-pipeline-tool update --pipeline "123456789012-customizations-pipeline" --trigger webhook

# ドライラン
./aft-pipeline-tool update --pipeline "123456789012-customizations-pipeline" --branch "develop" --dry-run
```

#### バッチ更新

```bash
# 全パイプラインのブランチをmainに変更（ドライラン）
./aft-pipeline-tool batch-update --branch "main" --dry-run

# 特定アカウントのみ更新
./aft-pipeline-tool batch-update --accounts "123456789012,234567890123" --trigger polling

# 実際に更新実行
./aft-pipeline-tool batch-update --branch "main" --trigger webhook
```

#### トリガー自動修正（fix-triggers）

```bash
# 全パイプラインのトリガーを自動修正（ドライラン）
./aft-pipeline-tool fix-triggers --dry-run

# 特定パイプラインのトリガーを修正
./aft-pipeline-tool fix-triggers --pipeline "123456789012-customizations-pipeline"

# 単一ソースアクションのみトリガーを設定
./aft-pipeline-tool fix-triggers --trigger-mode single --source-action "aft-global-customizations"

# 全トリガーを削除
./aft-pipeline-tool fix-triggers --trigger-mode none
```

#### ConnectionArn一括更新（fix-connections）

```bash
# 全パイプラインのConnectionArnを更新（ドライラン）
./aft-pipeline-tool fix-connections --connection-arn "arn:aws:codeconnections:region:account:connection/new-connection-id" --dry-run

# 特定パイプラインのConnectionArnを更新
./aft-pipeline-tool fix-connections --pipeline "123456789012-customizations-pipeline" --connection-arn "arn:aws:codeconnections:region:account:connection/new-connection-id"

# 特定ソースアクションのみConnectionArnを更新
./aft-pipeline-tool fix-connections --connection-arn "arn:aws:codeconnections:region:account:connection/new-connection-id" --source-action "aft-global-customizations"

# 実際に更新実行
./aft-pipeline-tool fix-connections --connection-arn "arn:aws:codeconnections:region:account:connection/new-connection-id"
```

#### パイプラインタイプ変更（fix-pipeline-type）

```bash
# 全パイプラインのPipelineTypeをV2に変更（ドライラン）
./aft-pipeline-tool fix-pipeline-type --pipeline-type V2 --dry-run

# 特定パイプラインのPipelineTypeを変更
./aft-pipeline-tool fix-pipeline-type --pipeline "123456789012-customizations-pipeline" --pipeline-type V2

# 全パイプラインをV1に変更
./aft-pipeline-tool fix-pipeline-type --pipeline-type V1

# 実際に更新実行
./aft-pipeline-tool fix-pipeline-type --pipeline-type V2
```

#### パイプライン実行管理（execute）

パイプラインの実行開始、停止、ステータス確認、履歴表示を行います。

##### パイプライン実行開始

```bash
# 単一パイプラインの実行開始
./aft-pipeline-tool execute start "123456789012-customizations-pipeline"

# 全AFTパイプラインの実行開始
./aft-pipeline-tool execute start --all

# 実行完了まで待機
./aft-pipeline-tool execute start "123456789012-customizations-pipeline" --wait

# 既に実行中でも強制実行
./aft-pipeline-tool execute start "123456789012-customizations-pipeline" --force

# タイムアウト設定（秒）
./aft-pipeline-tool execute start "123456789012-customizations-pipeline" --wait --timeout 7200
```

##### ファイルからの一括実行

```bash
# ファイルに記載されたパイプラインを一括実行
./aft-pipeline-tool execute start-from-file pipelines.txt

# 実行完了まで待機
./aft-pipeline-tool execute start-from-file pipelines.txt --wait

# シーケンシャル実行（1つずつ順番に実行し、完了を待ってから次を開始）
./aft-pipeline-tool execute start-from-file pipelines.txt --sequential

# 強制実行（既に実行中のパイプラインもスキップせずに実行）
./aft-pipeline-tool execute start-from-file pipelines.txt --force
```

**実行モード**:
- **並行実行（デフォルト）**: `max_concurrent`設定に従ってバッチ処理
  - 設定された数のパイプラインを同時に開始
  - 現在のバッチが完了してから次のバッチを開始
  - リソース効率的で安全な実行方式
- **シーケンシャル実行**: `--sequential`フラグまたは設定で有効化
  - 1つのパイプラインが完了してから次を開始
  - 確実だが時間がかかる方式

##### パイプライン実行停止

```bash
# 最新の実行中パイプラインを停止
./aft-pipeline-tool execute stop "123456789012-customizations-pipeline"

# 特定の実行IDを停止
./aft-pipeline-tool execute stop "123456789012-customizations-pipeline" "execution-id-12345"

# 停止理由を指定
./aft-pipeline-tool execute stop "123456789012-customizations-pipeline" --reason "Manual stop for maintenance"

# 実行IDフラグで指定
./aft-pipeline-tool execute stop "123456789012-customizations-pipeline" --execution-id "execution-id-12345"
```

##### パイプライン実行ステータス確認

```bash
# 特定パイプラインのステータス確認
./aft-pipeline-tool execute status "123456789012-customizations-pipeline"

# 全AFTパイプラインのステータス確認
./aft-pipeline-tool execute status

# 詳細情報を表示
./aft-pipeline-tool execute status "123456789012-customizations-pipeline" --details
```

##### パイプライン実行履歴表示

```bash
# 特定パイプラインの実行履歴（デフォルト10件）
./aft-pipeline-tool execute history "123456789012-customizations-pipeline"

# 全AFTパイプラインの実行履歴
./aft-pipeline-tool execute history

# 表示件数を指定
./aft-pipeline-tool execute history "123456789012-customizations-pipeline" --max-results 20

# 詳細情報を表示
./aft-pipeline-tool execute history "123456789012-customizations-pipeline" --details
```

##### パイプラインリストファイルの形式

`pipelines.txt`の例:
```
123456789012-customizations-pipeline
234567890123-customizations-pipeline
345678901234-customizations-pipeline
# コメント行（#で始まる行は無視されます）
456789012345-customizations-pipeline
```

#### テスト機能

```bash
# トリガー機能のテスト
./aft-pipeline-tool test-triggers --file pipeline.json

# ConnectionArn更新機能のテスト
./aft-pipeline-tool test-connections --file pipeline.json --connection-arn "arn:aws:codeconnections:region:account:connection/test-connection-id"
```

### 設定ファイル

設定ファイルは以下の場所に配置できます：
- `$HOME/.aft-pipeline-tool.yaml`
- `./config.yaml`
- `--config` フラグで指定

#### 設定例

```yaml
cache:
  directory: ~/.aft-pipeline-tool/cache
  accounts_ttl: 3600
  pipelines_ttl: 1800

exclude_accounts:
  - "111111111111"  # 管理アカウント
  - "222222222222"  # ログアーカイブアカウント

aws:
  region: us-east-1
  profile: default

output:
  format: table
  color: true

batch_update:
  dry_run: true

execution:
  max_concurrent: 3    # 最大同時実行数
  max_pipelines: 50    # ファイルから読み込む最大パイプライン数
  timeout: 3600        # 実行完了待機のタイムアウト（秒）
  sequential: false    # シーケンシャル実行モード
```

## 必要なIAM権限

このツールを使用するには、以下のAWS IAM権限が必要です：

- Organizations API（アカウント一覧取得）
- CodePipeline API（パイプライン操作・実行管理）

詳細な権限設定については、[仕様書](docs/spec.md#セキュリティ考慮事項)を参照してください。

## トラブルシューティング

### よくある問題

1. **AWS認証エラー**
   - AWS CLIの設定を確認してください
   - 適切なプロファイルが設定されているか確認してください

2. **権限不足エラー**
   - 上記のIAM権限が付与されているか確認してください

3. **パイプラインが見つからない**
   - AFTパイプラインの命名規則（`*-customizations-pipeline`）に従っているか確認してください

### ログとデバッグ

```bash
# 詳細なログを表示
./aft-pipeline-tool list --verbose

# キャッシュをクリア
rm -rf ~/.aft-pipeline-tool/cache
```

## 開発

### テスト実行

```bash
go test ./...
```

### ビルド

```bash
# 現在のプラットフォーム用
go build -o aft-pipeline-tool

# クロスコンパイル（Linux用）
GOOS=linux GOARCH=amd64 go build -o aft-pipeline-tool-linux

# クロスコンパイル（Windows用）
GOOS=windows GOARCH=amd64 go build -o aft-pipeline-tool.exe
```

## ライセンス

MIT License

## 貢献

プルリクエストやイシューの報告を歓迎します。

## サポート

問題が発生した場合は、GitHubのIssuesページで報告してください。
