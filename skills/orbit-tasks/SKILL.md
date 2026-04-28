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
orbit task create "タスクタイトル" --source <decision-id>

# Thread由来（議論を進めるための調査・作業）
orbit task create "タスクタイトル" --source <thread-id>

# 独立タスク
orbit task create "タスクタイトル" --priority h
```

`--source` はDecision/Threadのstable IDを直接渡す（`thread:` `decision:` のプレフィックスは不要、CLIが自動で種別判定する）。

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

### Task実行とgit連携

実装作業を伴うTaskは `orbit task start` で着手する。startは現在のgit branch名をTaskに記録し、以降そのbranchで作られるcommitをTaskと紐づけるアンカーになる。

```bash
orbit task start <task-id>   # 現在のbranchをtask.git_branchに記録
orbit task done  <task-id>   # task.git_branch上のcommit群をscanしてCommitエンティティ化＋紐づけ
```

- `orbit task start` 実行前にimplementation用branchへ切り替えておくこと。mainで開始するとmain上のcommitが全部紐づき得る
- `orbit task done` は `orbit task update --status done` のエイリアスかつcommit回収scanを走らせる。doneにはこちらを使う
- 1 commit : 1 taskが原則。同一branchを複数Taskで使い回すと紐づけが曖昧になるため避ける

### Commit-Task紐づけの救済

scanで取り逃した／別branchで作業した／cherry-pickした等で紐づけ漏れが起きた場合は手動で繋ぐ:

```bash
orbit commit list --task <task-id>     # taskに紐づくcommit一覧
orbit commit list                       # プロジェクト全commit一覧
orbit commit bind <sha> <task-id>       # 手動紐づけ
orbit commit unbind <sha>               # 紐づけ解除
orbit sync                              # gitリポジトリをscanしてcommitをtaskに紐づける（取りこぼし救済）
```

Orbitはgit hookを使わずpull型scanでcommitを取り込む設計。startで宣言→doneでscan、漏れたら bind で救済、というフローが基本。

### 注意

- Taskの完了はDecisionではない。`orbit task done` または `orbit task update --status done` を使う
- 作業中に設計上の選択肢が生まれたら、Taskの更新ではなくThreadを起票する
- source（Decision/Thread）を指定すると、DAGビューでTaskが接続され文脈が追跡可能になる
