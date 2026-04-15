---
id: "BL-001"
title: "Web UIへのロゴ・ファビコン設置"
type: build
status: open
priority: medium
labels: [web, branding]
created: 2026-04-15
updated: 2026-04-15
resolved: null
resolution: null
---

# Web UIへのロゴ・ファビコン設置

## 概要

orbit-logo.png / orbit-title.png をWeb UIに組み込む。ヘッダーロゴとファビコンの設置。

## 背景

ロゴとタイトル画像が作成された。Web UIは別セッションで実装中のため、そちらでブランディング要素を組み込む。

対象ファイル:
- `orbit-logo.png` — アイコン用（ファビコン、ナビゲーションバー等）
- `orbit-title.png` — ヘッダー用（ランディングページ、ログイン画面等）

やること:
- ファビコン生成（orbit-logo.png から 16x16, 32x32, 180x180 等）
- ヘッダーコンポーネントに orbit-title.png または orbit-logo.png + テキストを配置
- OGP画像の設定
