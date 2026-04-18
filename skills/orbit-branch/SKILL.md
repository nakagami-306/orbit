---
name: orbit-branch
description: >
  TRIGGER: ユ���ザーが設計の並行探索、ブランチ操作、マージ、コンフリクト解決をしたい場合。
  「ブランチ」「並行して検討」「マージ」「コンフリクト」「分岐」等のキーワード。
allowed-tools:
  - Bash
  - Read
---

# /orbit-branch — 並行設計探索

## ブランチの概念

ブランチはDecision DAG上のheadポインタ。設計の並行探索に使う。
- mainブランチ: プロジェクトの正式な設計状態
- 作業ブランチ: 試行錯誤・代替案の探索用

## ワークフロー

### ブランチ作成

```bash
orbit branch create              # 匿名ブランチ（デフォルト）
orbit branch create -n "名前"    # 名前付きブランチ
orbit branch list                # 一覧
orbit branch switch <name-or-id> # 切り替え
```

### ブランチ上での作業

通常の `/orbit-evolve` `/orbit-discuss` と同じ。ブランチ上のDecisionは自動的にそのブランチに属する。

### マージ

```bash
orbit branch merge <source-branch>  # 現在のブランチに source をマージ
```

3-way merge: fork point をbaseとし、base/source/target の3点比較。

- 片方だけ変更 → 変更側を自動採用
- 両方変更で同内容 → 自動採用
- 両方変更で異内容 → コンフリクト

### コンフリクト解決

```bash
orbit conflict list              # 未解決コンフリクト一覧
orbit conflict show <id>         # 詳細（両側の内容）
orbit conflict resolve <id> --resolution "選択内容" --rationale "なぜこちらを選んだか"
```

### ブランチ放棄

```bash
orbit branch abandon <name-or-id>
```

履歴は残る（却下した設計案の記録として）。
