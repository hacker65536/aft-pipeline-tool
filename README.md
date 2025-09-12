# AFT CodePipeline Git Triggers 編集ツール

AWS Control Tower Account Factory for Terraform (AFT) のCodePipelineにおけるGitトリガー設定を管理するGoツールです。

## 概要

このツールは、AFT管理下の複数アカウントのCodePipelineのGitトリガー設定を一括で確認・編集することができます。

## AI開発について

このツールは主にAI（Claude）によって開発されました。要件定義から実装、テスト、ドキュメント作成まで、開発プロセスの大部分がAIによって自動化されています。これにより、短期間での高品質なツール開発を実現しています。

## 機能

- AFTパイプライン一覧の表示
- 個別パイプラインのトリガー設定更新
- 複数パイプラインの一括更新
- パイプライントリガーの自動修正（fix-triggers）
- パイプラインConnectionArnの一括更新（fix-connections）
- パイプラインタイプの変更（fix-pipeline-type）
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
```

## 必要なIAM権限

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
