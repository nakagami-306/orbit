---
name: orbit-init
description: >
  TRIGGER: ユーザーがOrbitで新プロジェクトを初期化したい、または既存プロジェクトにディレクトリをリンクしたい場合。
  「orbit init」「プロジェクト作成」「Orbitで管理開始」等のキーワード。
allowed-tools:
  - Bash
  - Read
  - Write
---

# /orbit-init — プロジェクト初期化

## 前提確認

1. `orbit` コマンドが利用可能か確認: `orbit --version`
2. カレントディレクトリに `.orbit/` が既に存在するか確認

## ワークフロー

### 新規プロジェクト作成

```bash
orbit init <project-name>
```

- .orbit/ ディレクトリが生成される
- root Decision が自動作成される（全Decisionの起点）
- mainブランチが自動作成される

### 既存プロジェクトへのリンク

```bash
orbit init --link <existing-project-name>
```

### 初期化後

1. `orbit status` で健全性を確認
2. 初期のState（設計概要）を記述する場合は `/orbit-evolve` で Section を作成
