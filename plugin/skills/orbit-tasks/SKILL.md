---
name: orbit-tasks
description: >
  TRIGGER: ユーザーがタスクの作成、一覧、更新をしたい場合。実装作業の管理。
  「タスク」「task」「やること」「TODO」「実装する」「着手」「完了」等のキーワード。
allowed-tools:
  - Bash
  - Read
---

# /orbit-tasks — タスク管理

## Taskの位置づけ

Taskは「実行可能なアクション」。設計判断（Decision）でも議論（Thread）でもない。

- 実装完了 → Task を done にする（Decisionではない）
- 新しい作業が必要 → Task を作成する
- 作業中に設計上の問いが生まれた → `/orbit-discuss` に切り替える

## ワークフロー

### Task作成

```bash
# Decision由来（設計が決まり、それを実行する）
orbit task create "タスクタイトル" --source decision:<decision-id>

# Thread由来（議論を進めるための調査・作業）
orbit task create "タスクタイトル" --source thread:<thread-id>

# 独立タスク
orbit task create "タスクタイトル" --priority high --description "詳細"
```

### Task一覧

```bash
orbit task list                       # 全タスク
orbit task list --status todo         # 未着手のみ
orbit task list --status in-progress  # 進行中のみ
```

### Task更新

```bash
orbit task update <task-id> --status in-progress  # 着手
orbit task update <task-id> --status done          # 完了
orbit task update <task-id> --status cancelled     # キャンセル
```

### 注意

- Taskの完了はDecisionではない。`orbit task update --status done` を使う
- 作業中に設計上の選択肢が生まれたら、Taskの更新ではなくThreadを起票する
- source（Decision/Thread）を指定すると、DAGビューでTaskが接続され文脈が追跡可能になる
