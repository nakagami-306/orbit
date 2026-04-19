<!-- orbit:generated | 2026-04-19T00:27:21Z | branch:main | head:019da322-8440-7848-83cd-db009ec3af03 -->
# orbit

> あらゆる種類のプロジェクトの設計状態・意思決定・進行をバージョン管理するCLIツール。人間とAIの両方が同じインターフェースで操作できる。

## プロジェクト定義

ローカル環境内の全プロジェクトを中央DBから横断的に管理するCLIツール。Linear/Jira等の既存ツールがコード中心（Issue→PR→Deploy）であるのに対し、Orbitはプロジェクト中心——設計・リサーチ・議論・意思決定という、実際に時間を費やすフェーズをファーストクラスで扱う。実行フェーズの手段は問わない（コード、物理作業、手続き、AI）。

現在のdevlog/backlog運用の課題を解決するために構想された:
- ドキュメントの書き方ルールでしかなく、運用がプロジェクトごとに揺れる → CLIでシステマティックに管理
- バックログは個別のバグ・機能管理には適すが、設計議論や意思決定の変遷・分岐・収束の管理には適さない
- プロジェクト全体の「現在の設計状態」をトラッキングする仕組みがなく、手動スナップショット（v1, v2...）に頼っていた

## データモデル: エンティティ

### Project

取り組みの最上位コンテナ。ソフトウェアに限らない。

- id: stable UUID（不変）
- name, description, tags
- status: active / paused / archived

### State と Section

Projectは単一の「State」を持つ。これがプロジェクトの設計全体像を表す生きたドキュメント。orbit show で1つのドキュメントとして閲覧できる。

Stateは任意でSection（名前付きの区画）に分割できる。初期段階では単一テキストでよく、構造化が必要になった時点でSection化する（Section化自体がDecisionとして記録される）。

- Section: id, title, content, position, references（他Sectionへの依存リンク）
- Sectionはフラットリスト（木構造ではない）
- Section間のreferencesにより、あるSectionが変更されたとき依存先Sectionのstale検出が可能

### Decision

設計変更の原子単位。Datomicのトランザクションに対応し、JujutsuのCommitに相当する。複数Sectionを横断して原子的に変更する。

- id: stable UUID
- title: 何を決めたか
- rationale: なぜそう決めたか
- context: 決定時の前提条件
- parents: ref→Decision[]（DAG。通常1つ、マージ時2つ。root Decision以外は必須。parentなしDecisionはorbit init時に自動生成されるroot Decisionのみ）
- source_thread: ref→Thread | null
- author: 人間名 / AI / 共同

1 Decision = 1 DatomicトランザクションとしてEAVT datomsを生成。revert時はDecision全体を巻き戻すため、Section間の整合性が保証される。

### Topic

関連するThread群を事後的に紐づけるテーマ単位。Threadは起票時に全体像が見えないことが多く、調査・議論が進む中で複数Threadが同一テーマの別側面だと判明する。Topicはその構造が見えた時点で作成し、既存Threadを紐づける。

- id: stable UUID
- title: テーマ名（書き換え可能。問いが明確になるにつれ更新）
- description: 背景・文脈（任意）
- status: open / decided / abandoned
- outcome_decision: ref→Decision | null
- threads: N:N（中間テーブルtopic_threads）。1 Threadが複数Topicに属することを許容

Topic内のThreadに役割は持たせない。Topicは「テーマ」であり「問い」ではない（questionを持たない）。

Decision作成: orbit decide --source-topic でTopic単位のDecisionを作成。Thread→Decision経路は従来通り維持。

### Thread

議論・リサーチ・検討の場。Decisionに収束するか放棄される。

- id, title, question（何を検討しているか）, status（open / decided / abandoned）
- outcome_decision: 結論としてのDecision

内部にEntry（構造化された記録単位）を持つ:
- type: note / finding / option / argument / conclusion
- option: 検討中の選択肢
- argument: 特定のoptionに対する賛否。target（ref→Entry）とstance（for / against / neutral）で構造化
- finding: 調査で判明した事実
- note: 一般メモ
- conclusion: 結論（Decisionへの橋渡し）

### Task

実行可能なアクション。実行主体を問わない。

- id, title, description, status（todo / in-progress / done / cancelled）, priority, assignee
- source: ref→Decision | ref→Thread | null
  - Decision由来: 設計が決まりそれを実行するTask
  - Thread由来: 議論を進めるための調査・作業Task
  - null: 独立したTask

### Milestone

Decision DAG上の名前付きポインタ。Decisionのtag属性で代替可能だが、Milestone固有のメタデータ（description等）を持てるよう独立エンティティとしている。

- id, title, description
- decision: ref→Decision（この時点を指す）

### エンティティ間の関係

Project
 └── State ─── Section[] (フラットリスト、相互にreferences可能)
                   ▲
                   │ 原子的に変更
 └── Decision[] (DAG)
       ▲ 収束                    │ 派生
 └── Topic[] ←N:N→ Thread[]      Task[]
                     └── Entry[]

 └── Milestone[] ──→ Decision (ポインタ)

中心にいるのはDecision。ThreadはDecisionに直接収束するか、Topicを経由して収束する。Topicは関連Thread群を事後的にまとめるテーマ単位で、Topic単位のDecisionにより断片的な意思決定を統合する。TaskはDecisionまたはThreadから生まれ、MilestoneはDecisionを指す。

## ブランチモデル

ブランチはDecision DAG上のheadポインタ。意味的なスコープは持たない（gitブランチと同じ）。

- id, name（null可＝匿名）, head（ref→Decision）, status（active / merged / abandoned）
- mainブランチ: 各Projectに1つ。プロジェクトの正式な設計状態。orbit show のデフォルト
- 匿名がデフォルト: 名前は後から付けられる
- DAG上の任意のDecisionから分岐可能。ブランチからブランチも可
- マージ: 2つの親を持つDecisionを生成。任意の2ブランチ間でマージ可能
- **マージ戦略: 3-way merge**。fork point（分岐元のDecision時点のSection状態）をbaseとし、base/source/targetの3点比較で差分判定する:
  - base→source未変更、base→target変更 → target採用（自動）
  - base→source変更、base→target未変更 → source採用（自動）
  - 両方未変更 → そのまま（自動）
  - 両方変更かつ同内容 → どちらでもOK（自動）
  - 両方変更かつ異内容 → コンフリクト
- fork pointの特定: CreateBranch時にコピーしたhead_decision_idのtx_idからEntityStateAsOfでbase状態を復元
- コンフリクト: 同じSectionが両ブランチでbaseから変更された場合のみ、First-class Conflictとして記録。解決を強制しない
- 放棄: 履歴は残る（却下した案の記録として）
- diffは任意の2点間（ブランチ間、Decision間、混合）で取得可能

## CLI関数設計

人間もAIも同じコマンド体系を使う。--content/--stdinでエディタ起動をスキップ、--format jsonでJSON出力。モード切替ではなく標準的なフラグ設計で両方カバーする。

共通フラグ: --format json（JSON出力）、--project <name>（省略時はcwdの.orbitから解決）、--at <decision|milestone>（時間旅行）、-b <branch>（ブランチ指定）

### ID表示

CLIの全出力でエンティティのstable_id（UUID v7フルフォーマット）をそのまま表示する。短縮表示は行わない。UUID v7の先頭8文字はミリ秒タイムスタンプであり同一ミリ秒に作成されたエンティティで衝突するため、Gitの短縮SHAとは異なり安全な短縮ができない。プレフィックスマッチによる検索（FindSectionByNameOrID等）は引き続きサポートする。

### Project操作
orbit init <name> / orbit init --link <name> / orbit project list / orbit project archive|pause|activate

### State閲覧
orbit show — Stateを1ドキュメントとして表示
orbit status — 健全性サマリー（stale sections, unresolved conflicts, open threads, pending tasks）
orbit diff <point-a> <point-b> — 任意の2点間のState差分

### State変更
orbit edit [-s section] -t <title> -r <rationale> [--content <text> | --old <text> --new <text>] — State編集→Decision生成。--old/--newで既存内容の部分置換が可能
orbit section add/remove/ref — Section操作
orbit decide <thread-id> -s <section> -t -r [--content | --old --new] — Thread収束。-sと内容変更は必須。全Decisionは必ずSectionを更新する

### Decision操作
orbit decision log / orbit decision show / orbit revert

### Branch操作
orbit branch create/list/switch/name/merge/abandon

### Conflict操作
orbit conflict list/show/resolve

### Thread操作
orbit thread create/list/show/add/close / orbit decide

### Task操作
orbit task create/list/update

### Milestone操作
orbit milestone set/list

### 横断操作
orbit log — 全プロジェクト横断のアクティビティログ

## ローカルプロジェクション (.orbit/)

各プロジェクトディレクトリに .orbit/ を配置。中央DBのプロジェクションをMarkdownファイルとしてレンダリングしたもの。人間がエディタで、AIがReadツールで読める。

project-root/
 └── .orbit/
      ├── config.toml   # プロジェクトIDと中央DBへの紐づけ
      └── state.md      # Project Stateの全体（orbit showと同じ内容。読み取り専用）

同期モデル:
- orbit CLIでの書き込み時: 同じコマンド内でDB更新と.orbit/state.md再生成を実行（自動。手動sync不要）
- 別ディレクトリからの--project指定更新時: DBのパスマッピングを参照し、該当ディレクトリの.orbit/も再生成
- orbit CLIでの読み取り時: DBとファイルのバージョンハッシュを比較し、不一致なら再生成してから表示（鮮度チェック）
- ファイル直接読み取り時: ファイル冒頭のタイムスタンプとDecision IDで鮮度を判断可能

.orbit/state.mdは読み取り専用。編集はorbit CLI経由で行う（Decisionのtitle/rationaleが記録されないため直接編集は禁止）。

orbit init <name>: プロジェクト作成+カレントディレクトリへの紐づけ+.orbit/生成。既存プロジェクトへのリンクは orbit init --link <name>。

## エッジケース方針

設計上のエッジケースに対する方針。

### State / Section
- 未分割Stateの部分編集: 許容。diff粒度が粗いのはトレードオフ。ブランチ利用時にコンフリクト頻発ならorbit statusでSection化を提案するが強制しない
- Section間の循環参照: 許容。stale伝播は直接参照元（深さ1）のみに限定
- 参照先Sectionの削除: 警告表示+参照をdanglingマーク。削除はブロックしない
- ブランチ間のstale: ブランチスコープ内に限定。マージまで互いに影響しない

### Decision
- 全Decisionは必ずいずれかのSectionを更新する（強制）。Section内容は累積ではなくスナップショット（上書き）。履歴はDecision DAGが担う
- 空のDecision: 生成しない。変更がなければスキップ
- merge Decisionのrevert: 補償Decisionを生成。ブランチstatusはmergedのまま維持（事実の歴史は変えない）
- revertのrevert: 自然に動く。特別な保証不要。EAVTのappend-onlyモデルで各revertが独立イベント
- 大量Section一括変更: Decision単位のrationaleのみ。サブコメントは設けない。詳細経緯はThread側の責務

### Branch
- current branch管理: ディレクトリごとに.orbit/config.toml内で管理（Jujutsuのworkspace相当）
- 放棄ブランチの子ブランチ: 独立して存続。orbit branch listで「親ブランチabandoned」の表示は出す
- コンフリクトを含むブランチからの分岐: コンフリクトを引き継ぐ（Jujutsu式）
- マージのbase解決: fork point（CreateBranch時のhead_decision_id）のtx_idからEntityStateAsOfでSection状態を復元。fork pointが見つからない場合は空文字列をbaseとする（＝全Sectionが「新規追加」扱い）
- 3-way以上のマージ: 2-parent限定。順次マージで対応

### Thread / Task
- 決定済みThreadの再オープン: 不可。新Threadを作成して既存Thread/Decisionを参照する
- 複数Threadが同じSectionに結論: 同一ブランチ上では後勝ち（時系列が線形）。別ブランチならマージ時にコンフリクト
- Thread Entryの誤り訂正: retract+新Entry。Entryは不変。retractされたEntryはthread showでグレーアウト表示
- source DecisionがrevertされたTask: 通知のみ。自動cancelしない（物理作業は取り消せないため）

### 横断
- プロジェクトのアーカイブ: 未完了項目を警告して確認。ブロックはしない。データは消えない
- 複数ディレクトリから同一プロジェクト: 各ディレクトリが独立workspace。current branchはworkspaceごと
- .orbit/が消えた場合: 次回orbit CLI実行時に中央DBから自動再生成

## 技術スタック

- 言語: Go
- CLIフレームワーク: cobra
- SQLite: modernc.org/sqlite（pure Go、CGO不要でクロスコンパイル容易）またはgo-sqlite3（CGO、性能重視時）
- 配布: シングルバイナリ

選定理由: CLIツールとしての実績（gh, docker, k8s系）、開発速度、AI支援との相性、pure Go SQLiteによるクロスコンパイルの容易さ。Web UIはReact + React Flow + Viteで構築し、go:embedでシングルバイナリに統合。

Rust不採用理由: 型安全性の恩恵はあるがMVP到達速度を優先。EAVTの型管理はGoのinterface+テストで十分カバー可能と判断。

## データモデル: ストレージとバージョニング

3つの先行概念を統合する:

| 関心事 | 採用する概念 |
|--------|-------------|
| データの記録方式 | Datomic式 EAVT datom（append-only） |
| 時間旅行・状態復元 | Datomic式 as-of クエリ |
| 変更の「なぜ」の記録 | Reified Transaction + Event Sourcing メタデータ |
| エンティティの同一性追跡 | Jujutsu式 Change ID（改訂しても安定） |
| 設計の並行探索 | Jujutsu式 First-class Conflicts + Anonymous Branches |
| 操作の取り消し・復元 | Jujutsu式 Operation Log |
| ストレージ | SQLite 1ファイル（中央DB。全プロジェクト横断） |

SQLiteスキーマは3層構成:

1. イベントストア（datoms + transactions + operations）: append-onlyの唯一の真実の源泉。entities テーブルでJujutsu式 stable ID（UUID v7）を管理。datoms の値はJSON-encodedで型タグ付き。transactions はDecisionと1:1対応し、ブランチIDを保持。operations でJujutsu式Operation Logを記録（undo/redo用）
2. プロジェクション（p_* テーブル群）: クエリ性能のためのマテリアライズドビュー。p_projects, p_sections, p_decisions, p_branches, p_threads, p_entries, p_tasks, p_milestones, p_conflicts 等。コマンド実行時にdatom追記と同一トランザクションで更新。orbit db rebuild-projectionsで全datoms から再構築可能（Event Sourcing の Complete Rebuild）
3. ワークスペース（workspaces テーブル）: ディレクトリ⇔プロジェクトの紐づけ。current_branch_id, state_hashを保持

不変性はDBトリガーで強制（datoms/transactions へのUPDATE/DELETE禁止）。SQLiteはWALモードで運用。

EntityStateクエリの順序保証: 同一tx内でretract→assertが行われた場合、ROW_NUMBER()のORDER BYにrowidを含めてassert（後の行）が優先されることを保証する。これがないとブランチheadの更新が失われ、Decision DAGが断絶する。

root Decision: orbit init時にroot Decision（title: Project created, author: system, parent: なし）を自動生成しブランチheadに設定する。以降の全Decisionはparentが必須（例外なし）。これによりDecision DAGは常に単一rootからの連結グラフになる。

Section間のstale検出: Decision適用時にp_section_refsを参照し、変更されたSectionの参照元をstaleフラグ付きでUPDATE。

## AI連携

CLI関数に対応するClaude Code Pluginを実装。.claude-plugin/ディレクトリに配置（Claude Code Plugin規約）。marketplace.jsonをリポジトリルートに配置し、/plugin marketplace add nakagami-306/orbit → /plugin install orbit@orbit でインストール可能。skillsはCLIコマンドの薄いラッパーではなく、ワークフロー単位で複数コマンドを組み合わせた設計。

### Plugin構造

```
.claude-plugin/
  plugin.json              マニフェスト
  marketplace.json         マーケットプレイス定義
hooks/                     Hook定義（3つ、pluginに同梱するがClaude Codeの自動検出は未対応）
  hooks.json               Hook定義JSON
  orbit-session-start.py   SessionStart: エンティティ意味論・行動ルール・現在状態を注入
  orbit-decide-guard.py    PreToolUse: orbit decide/editのコマンド検査
  orbit-pre-compact.py     PreCompact: 行動ルール再注入 + 未記録rescue
skills/                    8つのワークフローSkill
references/                エンティティ意味論リファレンス（将来のprogressive disclosure用）
```

**Hooks配信方式:** Claude Codeのpluginシステムはドキュメント上hooks/hooks.jsonの自動検出をサポートしているが、実際にはplugin経由のhooksは発火しない（2026-04時点）。そのためhooksはプロジェクトの.claude/settings.jsonに直接定義する。hookコマンド内でinstalled_plugins.json経由のパス解決を行い、pluginキャッシュ内のスクリプトを実行するため、plugin更新への追従性は維持。

### Hooks（3層構造）

| Hook | イベント | 方式 | 役割 |
|------|---------|------|------|
| orbit-session-start.py | SessionStart | command → additionalContext JSON | エンティティ意味論（Decision/Section/Thread/Entry/Topic/Task/Milestone）、行動振り分けフロー、Decisionガードレール、議論記録ルール、現在のopen threads/statusを注入 |
| orbit-decide-guard.py | PreToolUse(Bash) | command → exit 2 deny | orbit decide/editの実行前にregex検査。-s未指定、「〜を反映」title、「方針に基づき」rationale等をdeny |
| orbit-pre-compact.py | PreCompact | command → additionalContext JSON | 行動ルール再注入（圧縮でSessionStart内容が消える場合の保険）+ open threadの未記録議論rescue |

### Skills（8つ、ワークフロー指向）

1. /orbit-init — プロジェクト初期化
2. /orbit-overview — 状況把握・健全性確認
3. /orbit-evolve — State編集→Decision生成（セルフチェック手順、アンチパターン集付き）
4. /orbit-discuss — Thread作成→議論→Decision収束（Entry type判定テーブル、記録粒度ガイドライン付き）
5. /orbit-branch — 並行設計探索・マージ・コンフリクト
6. /orbit-history — 履歴調査・時間旅行・revert
7. /orbit-tasks — タスク管理
8. /orbit-milestone — マイルストーン管理

設計原則: エンティティ知識はSessionStartで注入（当面全載せ）、Skillsはワークフロー手順に集中。コードベース膨張時にreferences/分離（公式推奨のprogressive disclosure）に移行。

### Web UI（実装済み）

orbit ui でブラウザベースのダッシュボードを起動。シングルバイナリに統合（go:embed）。

技術スタック: React + React Flow（DAG描画）+ Vite / Go net/http APIバックエンド。Viteビルド出力をinternal/api/dist/に配置し、ビルドタグ(dev/!dev)で開発時はVite dev server、本番時はembed.FSを切り替え。

画面構成:
- プロジェクト一覧 — 全プロジェクト俯瞰、統計情報（decisions/sections/tasks/threads数）
- Decision DAGビュー — dagreによる自動レイアウト。Decision/Thread/Task/Sectionをノードとして描画。ノード選択でサイドパネルに詳細表示（rationale、変更diff、source thread、related tasks）
- 横断タスクボード — プロジェクト横断kanban（todo/in-progress/done）、UIからステータス変更可能

API設計（全14エンドポイント）:
- GET /api/projects, /api/projects/:id（summary stats付き）
- GET /api/projects/:id/dag?branch&depth（nodes/edges/branches/milestones/threads/tasks/sections/entityEdges一括返却）
- GET /api/projects/:id/decisions/:did?branch（before/afterペア形式のdiff + relatedTasks）
- GET /api/projects/:id/sections, /sections/:sid, /threads, /threads/:tid, /branches, /milestones, /conflicts
- GET /api/tasks（projectName JOINで横断取得）, PATCH /api/tasks/:id（ステータス更新）
- GET /api/health

サーバー管理: orbit ui（バックグラウンドデーモン起動+ブラウザopen）、orbit ui stop、orbit ui status。PIDファイルで管理。内部的に--daemonフラグ付きで自プロセスを再実行し、OSデタッチAPI(Windows: DETACHED_PROCESS, Unix: Setsid)で子プロセスを独立。ログは~/.orbit/ui.logへ出力。

### 展開ステータス

2026-04にグローバルCLAUDE.mdおよびsettings.jsonのrhizome系プロジェクト管理機能（devlog/backlog/Vault同期/関連スキル・フック）を完全に置換。rhizomeはPKMとして継続使用（capture/search-kbスキルは保持）。

### Hook配信の回避策（Claude Code #27398）

plugin hooksが発火しないバグ(#27398)の回避策として、orbit initでプロジェクトの.claude/settings.jsonにhooks設定を自動生成する。hookコマンドはpluginが配信済みのPythonスクリプトをinstalled_plugins.json経由で参照する。バグ修正後はsettings.json側を削除してplugin hooksに切り替え可能。

