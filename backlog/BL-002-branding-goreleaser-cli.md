---
id: "BL-002"
title: "GoReleaser・CLIバナーへのブランディング適用"
type: build
status: done
priority: medium
labels: [branding, cli]
created: 2026-04-15
updated: 2026-04-15
resolved: 2026-04-15
resolution: "GoReleaserのrelease header追加、CLIバナーにASCIIアートロゴ実装"
---

# GoReleaser・CLIバナーへのブランディング適用

## 概要

orbit-logo.png のノード接続モチーフをASCIIアート化してCLIバナーに、orbit-title.png をGoReleaserのリリースヘッダーに適用する。

## 背景

ロゴ・タイトル画像の作成に伴い、CLIツールとリリースページにブランディングを反映する。README以外の接点でもプロジェクトの視覚的アイデンティティを統一する。

## 作業記録

### 2026-04-15 22:40: GoReleaser設定とCLIバナー実装

#### 概要
- `.goreleaser.yaml` に `release.header` / `release.footer` セクションを追加し、GitHub Releasesにロゴ画像を表示
- `internal/cli/root.go` にASCIIアートバナーを追加、`orbit` 単体実行時に表示
- ノード接続モチーフ（o---O---o）とFIGlet風ORBITロゴを組み合わせたデザイン

#### 実装詳細
- GoReleaser: `release.header` でorbit-title.pngをGitHub raw URLから参照、`release.footer` でFull Changelogリンクを自動生成
- CLIバナー: `const banner` として定義、`root.Run` でサブコマンドなし時に表示 + `cmd.Usage()` で通常ヘルプも続行

#### 変更ファイル
- `.goreleaser.yaml`
- `internal/cli/root.go`
