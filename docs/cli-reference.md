# Orbit CLI Reference

全コマンドの引数・フラグの一覧。Orbitの概念や使い方を先に理解したい場合は [ガイド](guide.md) を参照。

---

## Global Flags

全コマンドで使用可能なフラグ。

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | | `text` | 出力形式。`text` または `json` |
| `--project` | | (auto) | プロジェクト名を指定。省略時はカレントディレクトリの `.orbit/config.toml` から自動解決 |
| `--branch` | `-b` | (current) | 操作対象のブランチ名またはID |
| `--at` | | | 時間旅行。Decision IDまたはMilestone名を指定し、その時点の状態で操作 |

---

## orbit init

プロジェクトの新規作成、または既存プロジェクトへのディレクトリ紐づけ。

### 新規作成

```
orbit init <name> [flags]
```

カレントディレクトリに `.orbit/` を生成し、中央DBにプロジェクトを登録する。

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | プロジェクト名 |

| Flag | Description |
|------|-------------|
| `--description <text>` | プロジェクトの説明 |

**例:**
```bash
orbit init "my-app" --description "ECサイトリニューアルプロジェクト"
```

### 既存プロジェクトへのリンク

```
orbit init --link <name>
```

カレントディレクトリを既存プロジェクトに紐づける。別ディレクトリから同じプロジェクトを操作する場合に使用。

| Flag | Description |
|------|-------------|
| `--link <name>` | リンク先のプロジェクト名 |

**例:**
```bash
orbit init --link "my-app"
```

---

## orbit show

プロジェクトのState（設計全体像）を1つのドキュメントとして表示。

```
orbit show [flags]
```

Sectionが存在する場合はSection単位で表示。`.orbit/state.md` の鮮度チェックを行い、必要に応じて再生成してから表示する。

**例:**
```bash
orbit show                    # 現在のブランチの最新State
orbit show -b exploration     # 特定ブランチのState
orbit show --at D-xxx         # 特定Decision時点のState
orbit show --format json      # JSON出力
```

---

## orbit status

プロジェクトの健全性サマリーを表示。

```
orbit status [flags]
```

表示項目:
- プロジェクト名・ステータス
- 現在のブランチ
- Section数（stale数を含む）
- Decision数
- Open中のThread数
- 未解決Conflict数
- 保留中のTask数

**例:**
```bash
orbit status
orbit status --format json
```

---

## orbit edit

Stateを編集し、Decisionとして記録する。

```
orbit edit [flags]
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--title` | `-t` | **Yes** | Decision のタイトル（何を変えたか） |
| `--rationale` | `-r` | **Yes** | Decision の理由（なぜ変えたか） |
| `--section` | `-s` | Conditional | 編集対象のSection名。Sectionが1つなら省略可。複数ある場合は必須 |
| `--content` | | Conditional | 新しい内容。省略時はエディタが起動（未実装、現在は必須） |

Sectionが1つも存在しない場合、自動的に「State」という名前のSectionが作成される。

**例:**
```bash
# 初回（Section未作成時）
orbit edit -t "初期設計" -r "基盤定義" --content "プロジェクトの概要..."

# Section指定
orbit edit -t "認証方式変更" -r "セキュリティ要件追加" -s "認証設計" --content "OAuth2 + JWT"

# ブランチ上で編集
orbit edit -b exploration -t "代替案" -r "比較検討" -s "アーキテクチャ" --content "マイクロサービス案"
```

---

## orbit section

### orbit section add

新しいSectionを追加する。Decisionとして記録される。

```
orbit section add <title> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `title` | Yes | Section名 |

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--content` | | **Yes** | Sectionの内容 |
| `--title` | `-t` | No | Decisionのタイトル（省略時: "Add section: {title}"） |
| `--rationale` | `-r` | No | Decisionの理由（省略時: "Added section {title}"） |
| `--position` | | No | 表示順序（省略時: 末尾に追加） |

**例:**
```bash
orbit section add "アーキテクチャ" --content "モノリスでMVP構築"
orbit section add "データモデル" --content "EAVT + SQLite" -t "データモデル追加" -r "設計を明記"
```

### orbit section log

指定Sectionを変更した全Decisionの履歴を表示。「なぜ今この内容になっているか」を辿る。

```
orbit section log <section> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `section` | Yes | Section名またはStable IDプレフィックス |

**例:**
```bash
orbit section log "認証設計"
orbit section log "認証設計" --format json
```

---

## orbit decision

### orbit decision log

Decision履歴を時系列（新しい順）で表示。

```
orbit decision log [flags]
```

**例:**
```bash
orbit decision log
orbit decision log --format json
```

### orbit decision show

Decisionの詳細を表示。変更内容（どのSectionがどう変わったか）とrationale。

```
orbit decision show <decision-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `decision-id` | Yes | Decision の Stable IDプレフィックス |

**例:**
```bash
orbit decision show 019d8dab
```

---

## orbit revert

Decisionを巻き戻す。指定Decisionの変更を打ち消す補償Decisionを新たに生成する。元のDecisionは履歴に残る（削除されない）。

```
orbit revert <decision-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `decision-id` | Yes | 巻き戻すDecision の Stable IDプレフィックス |

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--rationale` | `-r` | **Yes** | 巻き戻しの理由 |

**例:**
```bash
orbit revert 019d8dab -r "認証方式を再検討するため"
```

---

## orbit thread

### orbit thread create

新しい議論Threadを作成する。

```
orbit thread create <title> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `title` | Yes | 議論のタイトル |

| Flag | Short | Description |
|------|-------|-------------|
| `--question` | `-q` | 何を検討しているか |

**例:**
```bash
orbit thread create "認証方式の検討" -q "JWTかSessionか"
```

### orbit thread list

Thread一覧を表示。

```
orbit thread list [flags]
```

| Flag | Description |
|------|-------------|
| `--status <status>` | ステータスでフィルタ: `open`, `decided`, `abandoned` |

### orbit thread show

Thread内のEntry一覧を構造化表示。

```
orbit thread show <thread-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `thread-id` | Yes | Thread の Stable IDプレフィックス |

### orbit thread add

ThreadにEntryを追加する。

```
orbit thread add <thread-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `thread-id` | Yes | Thread の Stable IDプレフィックス |

| Flag | Required | Description |
|------|----------|-------------|
| `--type <type>` | **Yes** | Entry種別: `note`, `finding`, `option`, `argument`, `conclusion` |
| `--content <text>` | **Yes** | Entryの内容 |
| `--target <entry-id>` | argument時 **Yes** | 対象EntryのStable IDプレフィックス |
| `--stance <stance>` | argument時 **Yes** | 立場: `for`, `against`, `neutral` |

Entry種別の意味:
- **note**: 一般的なメモ・観察
- **finding**: リサーチの発見事項（「調べたらXだった」）
- **option**: 検討中の選択肢
- **argument**: 選択肢に対する賛否（`--target` + `--stance` 必須）
- **conclusion**: 結論（Decisionへの橋渡し）

**例:**
```bash
orbit thread add THREAD_ID --type option --content "JWT: ステートレス、スケーラブル"
orbit thread add THREAD_ID --type argument --target ENTRY_ID --stance for --content "SPAとの相性が良い"
orbit thread add THREAD_ID --type conclusion --content "JWTを採用する"
```

### orbit thread close

Threadを放棄する。

```
orbit thread close <thread-id> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--reason` | `-r` | 放棄理由 |

---

## orbit decide

Threadの結論をDecisionとして記録し、Stateを更新する。Thread→Decision収束+State編集を一括実行。

```
orbit decide <thread-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `thread-id` | Yes | 収束させるThread の Stable IDプレフィックス |

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--title` | `-t` | **Yes** | Decision のタイトル |
| `--rationale` | `-r` | **Yes** | Decision の理由 |
| `--section` | `-s` | No | 更新するSection名 |
| `--content` | | No | Sectionの新しい内容（`--section` と併用） |

**例:**
```bash
orbit decide THREAD_ID -t "JWT採用" -r "SPAとの相性を重視" -s "認証設計" --content "JWT + リフレッシュトークン"
```

---

## orbit branch

### orbit branch create

現在のブランチのheadから新しいブランチを作成。

```
orbit branch create [flags]
```

| Flag | Description |
|------|-------------|
| `--name <name>` | ブランチ名（省略時: 匿名ブランチ） |

**例:**
```bash
orbit branch create --name "microservice-idea"
orbit branch create   # 匿名ブランチ
```

### orbit branch list

ブランチ一覧を表示。現在のブランチに `*` マーク。

```
orbit branch list [flags]
```

### orbit branch switch

作業対象ブランチを切り替える。`.orbit/state.md` は切り替え先のブランチの内容で再生成される。

```
orbit branch switch <branch> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `branch` | Yes | ブランチ名またはStable IDプレフィックス |

### orbit branch name

匿名ブランチに名前をつける。

```
orbit branch name <branch> <name> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `branch` | Yes | 対象ブランチ（名前またはIDプレフィックス） |
| `name` | Yes | 新しい名前 |

### orbit branch merge

ソースブランチをターゲットブランチにマージする。同じSectionが両ブランチで変更されている場合、First-class Conflictとして記録される。

```
orbit branch merge <source> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `source` | Yes | マージ元ブランチ |

| Flag | Description |
|------|-------------|
| `--into <target>` | マージ先ブランチ（省略時: 現在のブランチ） |

**例:**
```bash
orbit branch merge microservice-idea              # 現在のブランチにマージ
orbit branch merge microservice-idea --into main   # mainにマージ
```

### orbit branch abandon

ブランチを放棄する。履歴は残る（却下した設計案の記録として）。

```
orbit branch abandon <branch> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--rationale` | `-r` | 放棄理由 |

---

## orbit conflict

### orbit conflict list

未解決コンフリクトの一覧を表示。

```
orbit conflict list [flags]
```

### orbit conflict show

コンフリクトの詳細を表示。base値と各ブランチの値を表示。

```
orbit conflict show <conflict-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `conflict-id` | Yes | Conflict の Stable IDプレフィックス |

### orbit conflict resolve

コンフリクトを解決する。解決内容でSectionを更新し、Decisionとして記録する。

```
orbit conflict resolve <conflict-id> [flags]
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--content` | | **Yes** | 解決後のSection内容 |
| `--rationale` | `-r` | **Yes** | 解決の理由 |

**例:**
```bash
orbit conflict resolve 019dxxxx --content "モノリス + API ゲートウェイ（ハイブリッド案）" -r "両案の利点を組み合わせ"
```

---

## orbit task

### orbit task create

タスクを作成する。

```
orbit task create <title> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `title` | Yes | タスク名 |

| Flag | Description |
|------|-------------|
| `--priority <h\|m\|l>` | 優先度（デフォルト: `m`） |
| `--assignee <name>` | 担当者 |
| `--source <id>` | 元となるDecisionまたはThreadのID |

**例:**
```bash
orbit task create "JWT認証を実装する" --priority h --assignee tanaka
```

### orbit task list

タスク一覧を表示。

```
orbit task list [flags]
```

| Flag | Description |
|------|-------------|
| `--status <status>` | ステータスでフィルタ: `todo`, `in-progress`, `done`, `cancelled` |
| `--assignee <name>` | 担当者でフィルタ |

### orbit task update

タスクのステータスや担当者を更新する。

```
orbit task update <task-id> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `task-id` | Yes | Task の Stable IDプレフィックス |

| Flag | Description |
|------|-------------|
| `--status <status>` | 新しいステータス: `todo`, `in-progress`, `done`, `cancelled` |
| `--assignee <name>` | 新しい担当者 |

**例:**
```bash
orbit task update 019dxxxx --status in-progress
orbit task update 019dxxxx --status done
```

---

## orbit milestone

### orbit milestone set

Decision DAG上の特定ポイントにMilestoneを設定する。

```
orbit milestone set <title> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `title` | Yes | Milestone名 |

| Flag | Description |
|------|-------------|
| `--at <decision-id>` | 対象Decision（省略時: 現在のhead） |
| `--description <text>` | 説明 |

**例:**
```bash
orbit milestone set "MVP仕様確定" --description "コア機能の設計完了"
```

### orbit milestone list

Milestone一覧を表示。

```
orbit milestone list [flags]
```

---

## orbit diff

任意の2点間のState差分をSection単位で表示する。ポイントにはブランチ名、Decision IDプレフィックス、Milestone名のいずれかを指定可能。

```
orbit diff <point-a> <point-b> [flags]
```

| Argument | Required | Description |
|----------|----------|-------------|
| `point-a` | Yes | 比較元（ブランチ名、Decision ID、Milestone名） |
| `point-b` | Yes | 比較先 |

| Flag | Description |
|------|-------------|
| `--section <name>` | 特定Sectionに絞り込み |

**例:**
```bash
orbit diff main exploration               # 2ブランチ間
orbit diff main microservice-idea --section "アーキテクチャ"  # Section絞り込み
```

---

## orbit log

全プロジェクト横断のアクティビティログを表示。Decision単位で時系列表示。

```
orbit log [flags]
```

| Flag | Description |
|------|-------------|
| `--since <date>` | 開始日（ISO 8601） |
| `--until <date>` | 終了日（ISO 8601） |
| `--project <name>` | プロジェクトでフィルタ |

**例:**
```bash
orbit log
orbit log --since 2026-04-01 --until 2026-04-14
orbit log --project "my-app"
```

---

## orbit project

### orbit project list

全プロジェクトを一覧表示。

```
orbit project list [flags]
```

| Flag | Description |
|------|-------------|
| `--status <status>` | ステータスでフィルタ: `active`, `paused`, `archived` |

---

## ID の指定方法

エンティティIDの引数（decision-id, thread-id, task-id, conflict-id 等）は**Stable IDのプレフィックス**で指定できる。

```bash
# フルID
orbit decision show 019d8dab-a90c-76ab-8a2e-20461ce715ae

# プレフィックス（一意に特定できる長さ）
orbit decision show 019d8dab-a90c
```

プレフィックスが複数のエンティティに一致する場合は最初に見つかったものが使用される。一意に特定するためにはより長いプレフィックスを指定する。

---

## 環境変数

| Variable | Default | Description |
|----------|---------|-------------|
| `ORBIT_DB` | `~/.orbit/orbit.db` | 中央データベースファイルのパス |
| `EDITOR` | (OS default) | `orbit edit` でエディタを起動する際に使用 |

---

## ファイル

| Path | Description |
|------|-------------|
| `~/.orbit/orbit.db` | 中央SQLiteデータベース（全プロジェクト横断） |
| `.orbit/config.toml` | ディレクトリとプロジェクトの紐づけ設定 |
| `.orbit/state.md` | Project Stateの自動生成Markdownファイル（読み取り専用） |
