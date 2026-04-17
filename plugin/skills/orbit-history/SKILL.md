---
name: orbit-history
description: >
  TRIGGER: ユーザーが過去の意思決定を調査したい、差分を見たい、特定時点の状態を確認したい、変更を取り消したい場合。
  「なぜこうなった」「いつ決まった」「履歴」「diff」「revert」「時間旅行」「ログ」等のキーワード。
allowed-tools:
  - Bash
  - Read
---

# /orbit-history — 履歴調査・時間旅行

## ワークフロー

### Decision履歴の確認

```bash
orbit decision log                    # Decision一覧（新しい順）
orbit decision log --format json      # JSON出力
orbit decision show <decision-id>     # 詳細（rationale, 変更内容）
```

### 時間旅行

```bash
orbit show --at <decision-id>         # 特定Decision時点のState
orbit show --at <milestone-name>      # Milestone時点のState
```

### 差分比較

```bash
orbit diff <point-a> <point-b>        # 任意の2点間のState差分
```

point には Decision ID、ブランチ名、Milestone名を指定可能。

### Section変更履歴

```bash
orbit section log <section-title>     # 特定Sectionの変更履歴
```

### 変更の取り消し

```bash
orbit revert <decision-id>            # 補償Decisionを生成（元のDecisionは残る）
```

revertは破壊的操作ではない。元のDecisionは履歴に残り、それを打ち消す新しいDecisionが生成される。

### 横断ログ

```bash
orbit log                             # 全プロジェクト横断のアクティビティログ
```
