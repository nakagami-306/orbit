<!-- orbit:generated | 2026-04-15T16:44:59Z | branch:main | head:019d9208-2083-74df-8c23-0fe2906f5c98 -->
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
- parents: ref→Decision[]（DAG。通常1つ、マージ時2つ）
- source_thread: ref→Thread | null
- author: 人間名 / AI / 共同

1 Decision = 1 DatomicトランザクションとしてEAVT datomsを生成。revert時はDecision全体を巻き戻すため、Section間の整合性が保証される。

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
       ▲ 収束              │ 派生
 └── Thread[] ── Entry[]    Task[]

 └── Milestone[] ──→ Decision (ポインタ)

中心にいるのはDecision。設計の変遷はDecisionの連鎖、Stateの状態はDecision時点で確定、ThreadはDecisionに収束、TaskはDecisionまたはThreadから生まれ、MilestoneはDecisionを指す。

## ブランチモデル

ブランチはDecision DAG上のheadポインタ。意味的なスコープは持たない（gitブランチと同じ）。

- id, name（null可＝匿名）, head（ref→Decision）, status（active / merged / abandoned）
- mainブランチ: 各Projectに1つ。プロジェクトの正式な設計状態。orbit show のデフォルト
- 匿名がデフォルト: 名前は後から付けられる
- DAG上の任意のDecisionから分岐可能。ブランチからブランチも可
- マージ: 2つの親を持つDecisionを生成。任意の2ブランチ間でマージ可能
- コンフリクト: 同じSectionが両方で変更された場合、First-class Conflictとして記録。解決を強制しない
- 放棄: 履歴は残る（却下した案の記録として）
- diffは任意の2点間（ブランチ間、Decision間、混合）で取得可能

## CLI関数設計

人間もAIも同じコマンド体系を使う。--content/--stdinでエディタ起動をスキップ、--format jsonでJSON出力。モード切替ではなく標準的なフラグ設計で両方カバーする。

共通フラグ: --format json（JSON出力）、--project <name>（省略時はcwdの.orbitから解決）、--at <decision|milestone>（時間旅行）、-b <branch>（ブランチ指定）

### Project操作
orbit init <name> / orbit init --link <name> / orbit project list / orbit project archive|pause|activate

### State閲覧
orbit show — Stateを1ドキュメントとして表示
orbit status — 健全性サマリー（stale sections, unresolved conflicts, open threads, pending tasks）
orbit diff <point-a> <point-b> — 任意の2点間のState差分

### State変更
orbit edit [-s section,...] -t <title> -r <rationale> [--content <text>] — State編集→Decision生成
orbit section add/remove/ref — Section操作

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
- 空のDecision: 生成しない。変更がなければスキップ
- merge Decisionのrevert: 補償Decisionを生成。ブランチstatusはmergedのまま維持（事実の歴史は変えない）
- revertのrevert: 自然に動く。特別な保証不要。EAVTのappend-onlyモデルで各revertが独立イベント
- 大量Section一括変更: Decision単位のrationaleのみ。サブコメントは設けない。詳細経緯はThread側の責務

### Branch
- current branch管理: ディレクトリごとに.orbit/config.toml内で管理（Jujutsuのworkspace相当）
- 放棄ブランチの子ブランチ: 独立して存続。orbit branch listで「親ブランチabandoned」の表示は出す
- コンフリクトを含むブランチからの分岐: コンフリクトを引き継ぐ（Jujutsu式）
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

選定理由: CLIツールとしての実績（gh, docker, k8s系）、開発速度、AI支援との相性、pure Go SQLiteによるクロスコンパイルの容易さ。

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

Section間のstale検出: Decision適用時にp_section_refsを参照し、変更されたSectionの参照元をstaleフラグ付きでUPDATE。

## AI連携

