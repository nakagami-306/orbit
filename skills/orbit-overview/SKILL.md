---
name: orbit-overview
description: >
  TRIGGER: ユーザーがプロジェクトの状況を把握したい場合。セッション開始時、作業前の状況確認。
  「状況確認」「今どうなってる」「orbit show」「orbit status」「stateを見せて」等のキーワード。
  .orbit/ が存在するプロジェクトでセッションが始まった場合にも自動発火を検討。
allowed-tools:
  - Bash
  - Read
---

# /orbit-overview — 状況把握・健全性確認

## ワークフロー

### 1. 健全性サマリー

```bash
orbit status
```

確認項目:
- stale sections（依存先が変更されたSection）
- unresolved conflicts（マージコンフリクト）
- open threads（未決の議論）
- pending tasks（未完了タスク）

### 2. State全体像

```bash
orbit show
```

プロジェクトの現在の設計状態を1ドキュメントとして表示。

### 3. 状況に応じた案内

| 状態 | 推奨アクション |
|------|---------------|
| stale section あり | 該当Sectionの確認・更新を提案。`/orbit-evolve` |
| open thread あり | 議論の継続を提案。`/orbit-discuss` |
| pending task あり | タスク着手を提案。`/orbit-tasks` |
| unresolved conflict あり | コンフリクト解決を提案。`/orbit-branch` |
| 全て正常 | 現在の設計状態を要約して報告 |

### 4. 特定時点の状態を見たい場合

```bash
orbit show --at <decision-id or milestone-name>
orbit diff <point-a> <point-b>
```
