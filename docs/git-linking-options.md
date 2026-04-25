# Git連携: Commit-Task紐づけ設計案の比較

## 前提: 決まっていること

- Commitを第一級エンティティとしてEAVTに取り込む
- 紐づけ先は Task（Decisionではない）
- git hookは使わない（pull型scan）
- rebase追跡は諦める（orphanedマーク）
- Orbit BranchとGit Branchは紐づけない（レイヤーが違う）

---

## 現状（git連携なし）

```
  Orbit の世界
  ════════════════════════════════════════════════

  Orbit Branch ──head──▶ Decision ──updates──▶ Section
                              │
                           source
                              │
                 Thread ◀─────┤
                              │
                         Task ◀┘
                          │
                          │   ← ここから先がない
                          ▼
                         ???

  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─

  git の世界（Orbitから見えない）
  ════════════════════════════════════════════════

  git branch ──▶ commit ──▶ commit ──▶ commit
```

---

## 案E'（前回の結論: workspace の current task で紐づけ）

```
  Orbit DB
  ════════════════════════════════════════════════

  Decision ──▶ Section
      │
   Task ◀──────── Commit (新エンティティ)
                    │
                    sha, message, author...

  紐づけの仕組み:
  ┌─────────────────────────────────────────────┐
  │  .orbit/config.toml                         │
  │  current_task = "TASK-A"   ← ここに状態を持つ │
  └─────────────────────────────────────────────┘
       │
       │ scan時に current_task を見て bind
       ▼
  Commit.task = TASK-A

  ╔══════════════════════════════════════════════╗
  ║  問題: worktree が消えると config.toml も消える ║
  ║                                              ║
  ║  worktree-A (.orbit/config: task=A)          ║
  ║      commit X → task A                       ║
  ║      commit Y → task A                       ║
  ║        ↓ worktree削除                        ║
  ║      config消滅                               ║
  ║        ↓ mainでscan                          ║
  ║      commit X → task = null ← 紐づけ喪失     ║
  ╚══════════════════════════════════════════════╝
```

**結論: 並列worktreeで破綻する。以下の案で解決を図る。**

---

## 案F: p_branch_bindings テーブル

紐づけ情報を専用のプロジェクションテーブルに持つ。

```
  Orbit DB
  ════════════════════════════════════════════════

  Decision ──▶ Section
      │
   Task ◀──── Commit
      ▲           │
      │           sha, message...
      │
  ┌───┴──────────────────────────────────┐
  │  p_branch_bindings  (プロジェクション層) │
  │  ┌──────────┬──────────┬──────────┐  │
  │  │git_branch│ task_id  │orbit_br  │  │
  │  ├──────────┼──────────┼──────────┤  │
  │  │feat/auth │ TASK-A   │ (null)   │  │
  │  │feat/cache│ TASK-B   │ (null)   │  │
  │  └──────────┴──────────┴──────────┘  │
  └──────────────────────────────────────┘

  scan時: commit の出自 branch → テーブル照合 → task_id 解決
```

|          | 評価 |
|----------|------|
| worktree | OK - DB に残る |
| EAVT整合 | NG - プロジェクション層のみ、datomsから再構築不可 |
| 柔軟性   | NG - 1 branch : 1 task 固定 |
| 複雑度   | 低 |

---

## 案G: Task に git_branch 属性を追加

紐づけ情報をTaskの属性として持つ。新テーブル不要。

```
  Orbit DB
  ════════════════════════════════════════════════

  Decision ──▶ Section
      │
   Task ◀──── Commit
   │  │           │
   │  git_branch  sha, message...
   │  ="feat/auth"
   │
   │  scan時: commit の出自 branch
   │          → p_tasks.git_branch と照合
   └─────────→ Commit.task = この Task

  ┌──────────────────────────────────────┐
  │  p_tasks                             │
  │  ┌────────┬───────────┬───────────┐  │
  │  │task_id │ title     │git_branch │  │
  │  ├────────┼───────────┼───────────┤  │
  │  │TASK-A  │ 認証実装   │feat/auth  │  │
  │  │TASK-B  │ キャッシュ │feat/cache │  │
  │  └────────┴───────────┴───────────┘  │
  └──────────────────────────────────────┘
```

|          | 評価 |
|----------|------|
| worktree | OK - DB に残る |
| EAVT整合 | OK - Task属性としてdatomsに記録 |
| 柔軟性   | NG - 1 branch : 1 task 固定 |
| 複雑度   | **最低** |

---

## 案H: Assignment エンティティ

Task と Commit の間に中間エンティティを挟む。

```
  Orbit DB
  ════════════════════════════════════════════════

  Decision ──▶ Section
      │
   Task ◀──── Assignment (新) ◀──── Commit
               │         │              │
               git_branch repo          sha, message...
               ="feat/auth"

  1 Task に複数 Assignment が可能:

   Task-A ◀─┬── Assignment (feat/oauth-v1, abandoned)
             │       ◀── commit X
             │       ◀── commit Y
             │
             └── Assignment (feat/oauth-v2, active)
                     ◀── commit Z
                     ◀── commit W
```

|          | 評価 |
|----------|------|
| worktree | OK - DB に残る |
| EAVT整合 | OK - 独立エンティティ、完全再構築可能 |
| 柔軟性   | OK - 1 task : N branches |
| 複雑度   | 高（エンティティ数増加、commit→taskが2ホップ） |

---

## 並列worktreeシナリオの比較

```
  ┌─────────────────────────────────────────────────┐
  │  Agent 1: worktree-A          Agent 2: worktree-B│
  │  git branch: feat/auth        git branch: feat/cache
  │  commit X, Y                  commit Z           │
  │       ↓                            ↓             │
  │       └──── main にマージ ─────────┘             │
  │                    ↓                              │
  │              orbit sync (mainで実行)              │
  └─────────────────────────────────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
     案E' (current)  案F/G/H (branch名)
     ┌──────────┐  ┌───────────────────┐
     │X → null  │  │X → feat/auth →TASK-A│
     │Y → null  │  │Y → feat/auth →TASK-A│
     │Z → null  │  │Z → feat/cache→TASK-B│
     │全部迷子  │  │全部正しく紐づく      │
     └──────────┘  └───────────────────┘
```

---

## 一覧比較

| 観点 | E' (current task) | F (bindings table) | G (Task属性) | H (Assignment) |
|------|-------------------|--------------------|--------------|----------------|
| 紐づけの拠り所 | .orbit/config.toml | 専用テーブル | Task属性 | 中間エンティティ |
| worktree耐性 | **NG** | OK | OK | OK |
| EAVT再構築 | OK | **NG** | OK | OK |
| 1 task : N branches | - | NG | NG | **OK** |
| エンティティ追加数 | Commitのみ | Commitのみ | Commitのみ | **Commit + Assignment** |
| commit→task距離 | 1ホップ | 1ホップ | 1ホップ | **2ホップ** |
| 実装コスト | 低 | 低 | **最低** | 高 |

---

## 未解決の共通課題

**マージ後のブランチ出自判定**: 全案に共通する実装上の壁。

```
  マージ前:                     マージ後:
  
  main:    A──B──C              main:    A──B──C──M (merge commit)
                                                 /
  feat:       D──E              feat:       D──E

  feat ブランチが削除された後、
  D, E が「feat/auth 由来」だと
  どうやって判定するか？
```

候補:
1. merge commit のメッセージ解析 (`Merge branch 'feat/auth'`) → 脆い
2. scan を**マージ前に実行**する運用ルール → 忘れたら終わり
3. `orbit sync` 時に全ブランチの tip を記録しておく → 先行記録方式
