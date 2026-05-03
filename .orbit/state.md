<!-- orbit:generated | 2026-05-02T21:54:58Z | branch:main | head:019deab0-0a75-7da5-b717-0f5b38aa9366 -->
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
- git_branch: string | null — このTaskの実装が行われているgit branch名。commit紐づけの管理キー

### Commit

gitのcommitをOrbit内で追跡する第一級エンティティ。Orbitは「gitの外側から観察する層」として、hookを使わずpull型scanでcommitを取り込む。

- id: stable UUID
- sha: git commit SHA（リポジトリ内でユニーク）
- repo: リポジトリパスまたはremote URL
- message: コミットメッセージ1行目
- author: コミット作者
- authored_at: コミット日時
- parents: 親commitのSHA群
- task: ref→Task | null（1 commit : 1 task。紐づけなしも許容）
- status: active / orphaned / superseded

scan方式: Orbitコマンド実行時にgit rev-list HEAD → DB照合 → 未登録commitのみエンティティ化。commitの所属git branch名とp_tasks.git_branchを照合してtask紐づけを解決。

紐づけの3層構造:
1. 主経路: orbit task done時に自動scan（ブランチ削除前にcommitを回収）
2. ベストエフォート: 任意のorbitコマンド実行時に自動scan（生きてるブランチのcommitをbind）
3. フォールバック: orbit commit bind <sha> <task-id> で手動紐づけ

rebase対応: 追跡しない。HEADから到達不能になったcommitはstatus=orphanedにマーク。必要なら手動で再bind。

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
orbit task start <task-id> — Taskを開始し、現在のgit branch名をtask/git_branchに記録（自動取得）
orbit task done <task-id> — Task完了。自動scanでブランチ上のcommitを回収・紐づけ

### Commit操作（git連携）
orbit sync — 手動でcommit scanを実行（通常はコマンド実行時に自動scan）
orbit commit bind <sha> <task-id> — commitをtaskに手動紐づけ
orbit commit unbind <sha> — commitのtask紐づけを解除
orbit commit list [--task <task-id>] — commit一覧（task指定でフィルタ）

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

### Commit / git連携
- rebase/amend: 追跡しない。HEADから到達不能になったcommitはstatus=orphanedにマーク。旧→新のマッピングは記録せず、必要ならorbit commit bindで手動再bind
- ブランチ削除後のcommit: task done時の自動scanで回収済みなら問題なし。未回収の場合はtask=nullで登録され、手動bindで救済
- 1 commit : 1 task: 強制。1つのcommitが複数taskにまたがる場合はtaskの粒度が間違っている（task分割で対応）
- git hook: 使わない。Orbitはgitの外側から観察する層。husky/lefthook等との競合を回避
- 並列worktree: git branch名がtask紐づけのキー。worktree消滅しても中央DBのtask/git_branch属性から紐づけ可能。同一git branchに複数taskは不可
- Orbit BranchとGit Branch: 紐づけない。Orbit Branchは設計DAGのhead、git branchは実装DAGのhead。レイヤーが異なる

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

Orbitは設計プロセス全体を扱うため、AI（特にClaude Code）が日常的にCLIを叩く前提で作られている。AI連携の目的は、AIがOrbitを「正しく」使えるようナビゲートすることであり、本Sectionではそのための仕組みを整理する。

### 設計思想: 強制ではなく摩擦削減＋ノンブロッキング警告

初期実装ではAI判定型のStop hookで「議論記録漏れ」をブロックしていた（commit 4273223で廃止）。判定精度の低さ・JSON失敗・二重警告などの問題で運用に耐えなかった。

数日の実運用で観察された問題は別の根を持っていた。AI判定の不正確さ以前に、(a)強制ブロック型のフィードバック自体がAIの行動を予測不能にし、(b)SessionStart指示文だけではAIが内容を内面化せずEntry記録の型選択や粒度がバラバラになり、(c)Sectionの読者モデル（チャットコンテキスト無し）を破った記述・Decision粒度の肥大化・Decide→Task連鎖の欠落が継続発生した。

Plugin v2 ではこれを「強制」ではなく「摩擦削減＋ノンブロッキング警告」の方針で解決する:

- Decisionに関わる強い構造的検査はPreToolUse(Bash) hookで実施。誤った呼び出しは deny して再試行を促す（regex/構造ベースの軽量判定なので、廃止前のAI判定型hookで問題になった精度低下は回避）
- Entry記録のタイミング/型選択など曖昧な領域は、Stop hookで構造的キーワード判定して block + reason を返し、AIに記録を促す。判定はAIに任せず正規表現＋件数のみ
- 記録自体の摩擦は orbit thread add の --auto-type（contentからのtype推論）等で下げ、自然な記録発生を狙う

配信機構としてはhooksを継続採用しMCP化はしない。MCP化はorbitの20-30コマンド分だけツール定義を量産する必要があり、CLIと二重メンテになる。一方hooksはPreToolUse(Bash)1点で全orbitコマンドをカバーできる。

### Plugin構造

```
.claude-plugin/
  plugin.json         マニフェスト
  marketplace.json    マーケットプレイス定義
hooks/
  orbit-session-start.py   SessionStart: エンティティ意味論・行動振り分け・読者モデル等を注入
  orbit-decide-guard.py    PreToolUse(Bash): orbit decide/edit の引数を構造的に検査
  orbit-pre-compact.py     PreCompact: SessionStart内容の再注入＋未記録議論rescue
  orbit-stop-nudge.py      Stop: 議論記録漏れの構造的キーワード判定（blocking nudge）
skills/                    8つのワークフローSkill
references/                リファレンス（progressive disclosure用）
```

### Hooks仕様

| Hook | イベント | 役割 | 検査/注入内容 |
|------|---------|------|---------------|
| orbit-session-start.py | SessionStart | コンテキスト注入 | エンティティ意味論（Decision/Section/Thread/Entry/Topic/Task/Milestone/Commit/Repo）、行動振り分け（Decide→Task連鎖を含む）、Decision粒度ヒューリスティック（3ヶ月後revert基準）、Section読者モデル（チャットコンテキスト無し読者）、Git連携の運用 |
| orbit-decide-guard.py | PreToolUse(Bash) | 構造的deny | orbit decide -s未指定、title「〜を反映/実装/修正/対応/追加」、rationale 15字未満や「方針に基づき」「実装した」「動作確認」、Section content先頭がいきなり箇条書き（読者モデル違反） |
| orbit-pre-compact.py | PreCompact | 再注入 | 行動ルール再注入＋open thread未記録議論のrescue |
| orbit-stop-nudge.py | Stop | blocking nudge | assistant本文400字以上＋議論キーワード2件以上＋同ターン中にorbit thread add呼び出し0件のとき block + reason で記録を促す。stop_hook_active=trueでループ防止 |

orbit-decide-guard.py と orbit-stop-nudge.py の判定はすべて regex/構造ベース（軽量）。AIに判定させないため、廃止前のStop hookで問題だった精度低下は構造的に発生しない。

Stop hookの判定方式: 当初「ノンブロッキングで次ターンcontextに警告注入」を検討したが、Claude Code Stop hookの仕様上、確実にAIへ記録を促す経路は decision="block" + reason のみ。見かけ上blockだが、AIが応答続行（記録）するnudgeとして機能する。誤検知時もループは stop_hook_active=true の検査で防がれる。

### CLI側の行動誘導

Hook層と並行して、CLI出力自体でAIを誘導する。

- `orbit decide` / `orbit edit`: Decision生成成功時、出力末尾に「派生Taskは？必要なら orbit task create --source-decision <id>」を表示。Decide→Task連鎖の欠落を防ぐ
- `orbit thread add --auto-type`: contentから entry type を推論（finding/option/argument/conclusion/note）。記録の摩擦を下げて自然な記録発生を促す。推論ミスの実害は型違いのみ

### Skills（8つ、ワークフロー指向）

1. /orbit-init — プロジェクト初期化
2. /orbit-overview — 状況把握・健全性確認
3. /orbit-evolve — State編集→Decision生成
4. /orbit-discuss — Thread作成→議論→Decision収束
5. /orbit-branch — 並行設計探索・マージ・コンフリクト
6. /orbit-history — 履歴調査・時間旅行・revert
7. /orbit-tasks — タスク管理
8. /orbit-milestone — マイルストーン管理

設計原則: エンティティ知識はSessionStartで注入（当面全載せ）、Skillsはワークフロー手順に集中。

### Web UI（実装済み）

orbit ui でブラウザベースのダッシュボードを起動。シングルバイナリに統合（go:embed）。

技術スタック: React + React Flow（DAG描画）+ Vite / Go net/http APIバックエンド。Viteビルド出力をinternal/api/dist/に配置し、ビルドタグ(dev/!dev)で開発時はVite dev server、本番時はembed.FSを切り替え。

画面構成:
- プロジェクト一覧 — 全プロジェクト俯瞰、統計情報（decisions/sections/tasks/threads数）
- Decision DAGビュー — dagreによる自動レイアウト。Decision/Thread/Task/Sectionをノードとして描画。ノード選択でサイドパネルに詳細表示
- 横断タスクボード — プロジェクト横断kanban、UIからステータス変更可能

API設計（全14エンドポイント）:
- GET /api/projects, /api/projects/:id（summary stats付き）
- GET /api/projects/:id/dag?branch&depth
- GET /api/projects/:id/decisions/:did?branch
- GET /api/projects/:id/sections, /sections/:sid, /threads, /threads/:tid, /branches, /milestones, /conflicts
- GET /api/tasks, PATCH /api/tasks/:id
- GET /api/health

サーバー管理: orbit ui（バックグラウンドデーモン起動+ブラウザopen）、orbit ui stop、orbit ui status。PIDファイルで管理。

### Hook配信の運用

Claude Code #27398 によりplugin同梱のhooks.jsonは自動検出されない（2026-04時点）。そのためhooks.json自体をリポジトリから削除し、orbit init が .claude/settings.json にhook定義を自動生成する方式に一本化した。生成されるコマンドは installed_plugins.json から orbit plugin の installPath を解決して plugin cache 内のPythonスクリプト（hooks/orbit-*.py）をexecで読み込む形のone-liner。plugin更新（marketplace update + reload-plugins）後も installPath 経由で最新版が呼ばれるため追従性は維持される。

クロスプラットフォーム対応: orbit init が runtime.GOOS で実行OSを判定し、Windowsは `python`、macOS/Linuxは `python3` を埋め込む。Windowsで `python3` を使うと Microsoft Store stub（C:\Users\...\WindowsApps\python3.exe）に当たり exit 49 で失敗するケースがあり、macOS最近版・Ubuntu 22.04+では `python` が標準で存在しないため、片方統一は不可。init時にOSが確定しているので分岐するのが素直。

過去の二次問題: 旧版（v0.3.0以前）のplugin cacheに残存していたprompt型Stop hookが他プロジェクトでも発火する問題があったが、Plugin v0.3.1（commit 7e7ce83）でcache更新により正式に解消した。

### 展開ステータス

2026-04にグローバルCLAUDE.mdおよびsettings.jsonのrhizome系プロジェクト管理機能（devlog/backlog/Vault同期）を完全に置換。rhizomeはPKMとして継続使用（capture/search-kbスキルは保持）。

Plugin v2 は2026-04にPhase 1（SessionStart刷新・DecideGuard強化・AI連携Section再構成）とPhase 2（Stop hook再導入・thread add type推論・decide/edit CLIでのTask提案）を完了。Plugin v0.4.2 で参考扱いだったhooks/hooks.jsonを削除し、orbit init による .claude/settings.json 生成時に runtime.GOOS で python/python3 を選択するクロスプラットフォーム対応を実装、現在 v0.4.2 として配信中。

エンティティ意味論の伝達経路はSessionStart hookに一元化する方針を確定済み。Skill個別注入は発火タイミング依存でカバレッジに穴が空くため副次的位置に留め、SessionStartが全エンティティ意味論（Decision/Section/Thread/Entry/Topic/Task/Milestone/Commit/Repo）と行動振り分け・読者モデル・粒度ヒューリスティックの単一窓口を担う。git連携の意味論注入もこの方針に基づきPlugin v2 Phase 1で完了。Skill側へのgit連携ワークフロー追記とWeb UIでのcommit-task紐づけ可視化はPhase 2残課題として独立Task化済み（019dd184-c382, 019dd184-c68e）。

## Git連携

Commit-Task紐づけ機構の設計。

Orbitは設計判断（Decision DAG）とコード実装（git commits）を別レイヤーで管理する。両者を完全に独立させると、commitを見ても背景の判断が辿れず、Decisionを見てもどの実装に反映されたかが分からない。Git連携はこの2レイヤーを橋渡しする層であり、gitのcommitをOrbit内の第一級エンティティとして取り込み、Taskを介してDecision DAGに間接接続する。

設計の核となる選択:

- **紐づけ先はDecisionではなくTask**: 粒度が合う（1 Task ≒ 数commits）。設計変更を伴わないcommit（typo修正・依存更新等）も独立Task（source: null）で自然にカバーできる。Decision直接だと空虚なDecision量産か未リンク放置になる
- **git hookは使わずpull型scan**: husky/lefthook等のhook管理ツールとの競合を回避、commit message規約への依存も回避。Orbitはgitの外側に座って観察する層という哲学に整合
- **rebaseは追跡しない**: HEAD到達不能commitはstatus=orphanedにマーク、旧→新マッピングは記録せず手動再bindに委ねる。post-rewrite hookの重さと多対一マッピングの表現困難への対応コストが益に見合わない
- **Orbit BranchとGit Branchは紐づけない**: Orbit Branchは設計DAGのhead、git branchは実装DAGのhead。レイヤーが異なるため独立に維持

### データモデル

新エンティティ Commit と Repo を導入:
- **Commit**: id, sha (repo内ユニーク), repo (ref→Repo), message, author, authored_at, parents (SHA配列), task (ref→Task | null), status (active / orphaned)
- **Repo**: id, uuid (内部識別キー、不変), remote_url (観測値、識別には使わない)
- **Task** に git_branch 属性を追加: scan時のtask検索キー
- **Workspace** に repo_root カラムを追加: scan対象の解決元

repo識別キーをremote URLや絶対パスでなくOrbit内部UUIDにする理由: remoteを持たないローカルrepo・remote付け替え・rename・別マシン移動に耐える。当面 1プロジェクト : 1 repo の制約、multi-repoは将来拡張。

### scan発火タイミング

全orbitコマンドのPersistentPreRunで自動scanを発火する。差分0件はno-opで即return。発火コストは極小（git rev-list 数十ms + DB照合のみ）のため、read/writeを区別する除外ロジックは設けない。scan中のエラーは警告ログのみで本処理はブロックしない（git未インストール、repo破損等でorbitが使えなくなる事態を回避）。

task done時の自動scanはこの汎用scanとは別経路で確実に実行する（doneブランチに限定したスコープ）。これは紐づけ3層構造の主経路を担う。

### scanの取得範囲とworktree

scanは `git rev-list --all` + `git for-each-ref refs/heads` で全ローカルbranch祖先を網羅する。

- 各worktreeから個別発火されてもDB照合（WHERE sha NOT IN (...)）で重複登録は弾ける
- worktree消滅後も中央DBにcommitが残るため取りこぼしなし
- commitの所属branch解決: for-each-refで取得したbranch tipから git rev-list で逆引きし、各commitの到達可能branch候補を得る
- 複数branchから到達可能な場合は p_tasks.git_branch=X AND status='in-progress' のtaskを優先
- orphaned判定は別フェーズ: 既登録commitのうち git rev-list --all から到達不能なものを再マーク

### task↔git_branch紐づけ制約

"1 git_branch : 1 task" は 同一プロジェクト + 同一repo + 同一branch名 + active(todo/in-progress) スコープで成立する。

orbit task start <id> 仕様:
- `git rev-parse --abbrev-ref HEAD` の結果を task/git_branch 属性に記録
- 同一プロジェクト+同一branch名で active なtaskが既にあればエラー
- detached HEAD（branch名が取れない状態）はエラー
- 既に git_branch が設定済みのtaskを再startしたらbranch名を上書き + warning表示
- doneしたtaskのbranch名は再利用可（古いcommitは既存taskに紐づいたまま、新commitが新taskに紐づく）

scan時のtask検索条件:
- `status='in-progress' AND git_branch=X` を最優先
- 次点で `status='todo' AND git_branch=X`
- done/cancelled は除外（古いcommitの誤再bind防止）
- 該当taskなしなら **p_commits に登録しない**（Orbit管轄外。gitに残るだけ）。後から重要と判明したcommitは `orbit commit bind <sha> <task-id>` で能動的に取り込む（bind時にgitから情報を引いてp_commits新規登録）

この方針はOrbitが『設計判断とgit実装の橋渡し層』であってcommit storage層ではないという責務分担に基づく。gitが第一級storageとして全commitを永続化しているため、橋渡し相手（task）のないcommitを冗長にOrbitへ登録するのはノイズにしかならない。

### 紐づけのフォールバック3層

1. **主経路**: orbit task done時の自動scan（ブランチ削除前にcommit回収）
2. **ベストエフォート**: 全orbitコマンド冒頭の自動scan（生きてるbranchのcommitをbind）
3. **フォールバック**: orbit commit bind <sha> <task-id> で手動紐づけ

