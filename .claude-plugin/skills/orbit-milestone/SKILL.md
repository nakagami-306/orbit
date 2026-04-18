---
name: orbit-milestone
description: >
  TRIGGER: ユーザーが重要な時点にマーカーを設定したい場合。リリースポイント、フェーズ完了の記録。
  「マイルストーン」「milestone」「リリース」「v1.0」「フェーズ完了」等のキーワード。
allowed-tools:
  - Bash
  - Read
---

# /orbit-milestone — マイルストーン管理

## Milestoneの位置づけ

Decision DAG上の名前付きポインタ。特定のDecision時点に名前を付けるマーカー。
時間旅行の基準点として使える（`orbit show --at <milestone-name>`）。

## ワークフロー

### Milestone設定

```bash
orbit milestone set "v0.1.0" --decision <decision-id> --description "初期データモデル確定"
```

### Milestone一覧

```bash
orbit milestone list
```

### Milestoneを使った時間旅行

```bash
orbit show --at "v0.1.0"
orbit diff "v0.1.0" "v0.2.0"
```
