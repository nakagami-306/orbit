# Orbit — CLAUDE.md

## プロジェクト概要

Go製のプロジェクト管理CLIツール。設計状態・意思決定・進行をEAVT（Entity-Attribute-Value-Transaction）モデルでバージョン管理する。

## ビルドとテスト

```bash
go build ./cmd/orbit/          # ビルド
go test ./internal/db/ -v      # DBレイヤーテスト
go test ./internal/eavt/ -v    # EAVTレイヤーテスト
go test ./... -v               # 全テスト
```

## アーキテクチャ

### ディレクトリ構成

```
cmd/orbit/main.go       エントリポイント（CLIをNewRootCmdで起動するだけ）
internal/
  db/                    SQLite接続・スキーマ・マイグレーション
    db.go                Open/Close/Tx — 全書き込みはTx()経由
    schema.go            全DDL（3層: イベントストア/プロジェクション/ワークスペース）
  eavt/                  コアデータレイヤー
    value.go             7型の値エンコーディング (JSON: {"t":"s","v":"..."})
    attributes.go        全属性名の定数 (project/name, section/content等)
    datom.go             Datom構造体 (E, A, V, Tx, Op)
    entity.go            エンティティ作成・検索・UUID v7生成
    transaction.go       EAVTトランザクション・datom assert/retract
    query.go             EntityState, EntityStateAsOf, EntitiesByAttribute
  projection/            EAVTからプロジェクションへの同期
    projector.go         ApplyDatoms — エンティティ種別ごとにp_*テーブルを更新
  domain/                ビジネスロジック
    project.go           ProjectService — CreateProject, EditSection, AddSection, GetSections
    thread.go            ThreadService — CreateThread, AddEntry, Decide, CloseThread
    branch.go            BranchService — CreateBranch, MergeBranch, SwitchBranch, AbandonBranch
    task.go              TaskService — CreateTask, ListTasks, UpdateTask
  workspace/             ローカルファイル管理
    workspace.go         DBPath, Resolve, Register — .orbit/config.toml管理
    renderer.go          RenderState — p_sectionsからstate.md生成
  cli/                   Cobraコマンド定義（1ファイル1コマンド群）
    root.go              ルートコマンド・グローバルフラグ・resolveProject
    init.go, show.go, status.go, edit.go, section.go, sectionlog.go
    decision.go, revert.go, thread.go, branch.go, conflict.go
    task.go, milestone.go, diff.go, log.go, project.go
```

### データフロー

全書き込みは単一パターン:
1. `db.Tx()` でSQLトランザクション開始
2. `eavt.CreateEntity()` / `eavt.AssertDatom()` / `eavt.RetractDatom()` でdatomsテーブルに追記
3. `projection.Projector.ApplyDatoms()` で該当のp_*テーブルを更新
4. `workspace.RenderState()` で.orbit/state.mdを再生成
5. コミット（全てが同一トランザクション内）

### 重要な設計判断

- **全State変更はDecisionを伴う**: editもsection addもDecisionエンティティを作成する。Decision = EAVTトランザクション
- **ブランチはheadポインタ**: p_sectionsの複合PK (entity_id, branch_id) でブランチ間のSection分離を実現
- **datomsは不変**: DBトリガーでUPDATE/DELETEを禁止。retractは新しいdatom (op=0) の追記
- **プロジェクションは再構築可能**: datomsから全p_*テーブルを再構築できる（Event Sourcingの Complete Rebuild）

### エンティティと属性

属性定数は `internal/eavt/attributes.go` に定義。エンティティ種別:
- project, section, decision, thread, entry, task, milestone, branch

### 中央DB

パス: `~/.orbit/orbit.db`（`ORBIT_DB`環境変数で変更可）。全プロジェクト横断の単一SQLiteファイル。

## 新しいコマンドの追加方法

1. `internal/cli/` に新ファイル作成
2. `func newXxxCmd(app *App) *cobra.Command` を実装
3. `root.go` の `NewRootCmd()` 内で `root.AddCommand(newXxxCmd(app))` を追加
4. ドメインロジックが必要なら `internal/domain/` に追加

## 新しいエンティティ属性の追加方法

1. `internal/eavt/attributes.go` に属性定数を追加
2. `internal/db/schema.go` の該当p_*テーブルにカラム追加（マイグレーション）
3. `internal/projection/projector.go` のapply*メソッドで新属性を処理
4. ドメイン層の構造体とCLIコマンドを更新

## テスト

テスト用DBは `t.TempDir()` にSQLiteファイルを作成して使う。`:memory:` ではなくファイルDBを使うのはWALモードの確認のため。

```go
func setupTestDB(t *testing.T) *db.DB {
    d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
    // ...
}
```

## Orbit の使い方（Claude向け）

このプロジェクト自体が Orbit で管理されている。作業中は以下を厳守する。

### Thread での議論記録

チャット上で議論が進んだら、**その都度** `orbit thread add` で記録する。結論だけでなく経緯も残す：
- 選択肢が出たら `--type option` で記録
- 事実や調査結果は `--type finding`
- 選択肢への賛否は `--type argument --stance for/against --target <entry-id>`
- 結論は `--type conclusion`
- 自由な議論メモは `--type note`

「後でまとめて記録」は禁止。議論のたびにリアルタイムで記録する。

### Decision と State の同期

**全 Decision は必ずいずれかの Section を更新しなければならない。**

- `orbit decide` では `-s`（section）と `--content`（セクション全文）を必ず指定する
- Section 内容は累積ではなく上書き（スナップショット）。そのトピックの「現在の設計状態」を全文リライトする
- 履歴は Decision DAG が担うので、Section には最新の状態だけあればよい

### 問題・仕様変更が発生したら

1. まず `orbit thread create` で起票する
2. チャットで議論しながら thread にエントリを記録する
3. 方針が決まったら `orbit decide` で収束させ、**必ず Section を更新する**

## Orbit Plugin（Claude Code統合）

このリポジトリはClaude Code Pluginを同梱している。`skills/`, `hooks/`, `references/` はルート直下、`plugin.json`, `marketplace.json` は `.claude-plugin/` に配置。marketplace経由でインストールして使用する。

Plugin内のファイルを変更した場合はセッション内で `/reload-plugins` を実行して反映する。

## 設計ドキュメント

詳細な設計仕様は `C:\Users\seito\Desktop\manager\` を参照:
- `state.md` — 全仕様（エンティティ、ストレージ、ブランチ、CLI、エッジケース）
- `backlog/BL-007-eavt-schema.md` — SQLiteスキーマの詳細設計
