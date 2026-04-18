# Orbit エンティティ意味論リファレンス

Skillsおよびhooksから参照される共通知識。Orbitの各エンティティが何であり、何でないか。いつ使い、いつ使わないか。

---

## Decision — 設計変更の原子単位

Decisionはプロジェクトの中心エンティティ。**設計上の岐路で選択肢から1つを選んだ記録**。

| 属性 | 意味 |
|------|------|
| title | 何を決めたか。意思決定の内容を端的に |
| rationale | なぜそう決めたか。代替案と比較しての理由 |
| context | 決定時の前提条件 |
| parents | DAGの親Decision（通常1つ、マージ時2つ） |
| source_thread / source_topic | この決定の議論元 |

**Decisionであるもの:**
- 「GoではなくRustを採用する。理由は〜」
- 「Section分割ではなく単一Stateで運用する。理由は〜」
- 「user_versionベースのマイグレーションを採用する。理由は〜」

**Decisionでないもの（過去のAIの誤用実例）:**
- 「パッチモードの動作確認を反映」← テスト結果であってDecisionではない
- 「Decisionは必ずSectionを更新する方針に基づき反映」← 既存決定の転記であって新たな意思決定ではない
- 「Task実装の完了を記録」← Task更新であってDecisionではない

**判定テスト（orbit decide / orbit edit の前に必ず）:**
1. 代替案が存在し、その中から1つを選んだか？
2. rationaleは「なぜAではなくBか」を説明しているか？
3. titleは「〜を決定」「〜を採用」であって「〜を反映」「〜を同期」ではないか？

→ 1つでもNoなら、Decisionではない。正しいエンティティに振り分ける。

**必須制約:** 全DecisionはいずれかのSectionを更新する。Sectionの内容はスナップショット（上書き）。Decision作成とSection更新は同一タイミングで完結させる。「後で反映」は禁止。

---

## Section — Stateの区画

プロジェクトの設計全体像（State）を構成する名前付き区画。**現在の設計状態のスナップショット**。

- 常に最新の状態だけを保持する。過去の状態はDecision DAGが担う
- 累積的な追記ドキュメントではない。Decisionのたびに該当箇所を上書きする
- フラットリスト。Section間のreferencesにより依存関係を表現
- 依存先が変更されるとstaleフラグが立つ

---

## Thread — 議論・検討の場

1つの問いに対する構造化された議論の場。**Decisionに収束するか放棄される**。

| 属性 | 意味 |
|------|------|
| title | 議論のテーマ |
| question | 何を検討しているか（1文） |
| status | open → decided / abandoned |
| outcome_decision | 収束先のDecision |

**Threadを作るべきとき:**
- 設計上の不確実性がある（正解がすぐにわからない）
- 選択肢が複数ある
- トレードオフの検討が必要
- 問題の原因調査が必要

**Threadを作らなくてよいとき:**
- 明らかに正しい修正（typo、バグ修正）
- ユーザーが直接「こうして」と指示した明確な変更

---

## Entry — Threadの構造化記録単位

Threadの中の1つ1つの記録。**議論の経緯を構造化して残す**。

| type | いつ使うか | 例 |
|------|-----------|-----|
| finding | 調査で事実が判明した | 「migrateはentitiesテーブル有無のみで判定している」 |
| option | 選択肢が提示された | 「案A: user_versionで管理」 |
| argument | 選択肢の賛否が論じられた | 「案Aの利点: SQLite標準、追加テーブル不要」 |
| conclusion | 方針が決まった | 「user_versionベースを採用」 |
| note | 上記どれにも当てはまらない | 「この問題は別threadで扱う」 |

**記録タイミング:** 議論の中で上記に該当する発言が出たら、その都度すぐに記録する。「後でまとめて」は禁止。チャットのコンテキストは圧縮で消えるが、Orbitの記録は残る。

**迷ったときの優先順位:** finding > option > argument > conclusion > note。事実の記録は最も安全。

---

## Topic — Thread群のテーマ単位

関連するThread群を事後的にまとめるテーマ。調査・議論が進む中で「これらは同じ話だ」と判明した時点で作成する。

- 事前の計画ではなく、事後的な構造化
- titleは問いが明確になるにつれ更新してよい
- Topic単位のDecisionで断片的な意思決定を統合できる

---

## Task — 実行可能なアクション

実行すべき作業。**設計判断や議論ではなく、やることリスト**。

| source | 意味 | 例 |
|--------|------|-----|
| Decision由来 | 設計が決まり、それを実行する | 「user_versionマイグレーション実装」 |
| Thread由来 | 議論を進めるための調査・作業 | 「既存DBのスキーマバージョン確認」 |
| null | 独立したTask | 「READMEの更新」 |

**重要:** 実装の完了はTask更新（`orbit task update --status done`）であって、Decisionではない。

---

## Milestone — Decision DAGのマーカー

特定のDecision時点に名前を付けるポインタ。時間旅行の基準点。

---

## 行動振り分けフロー

何かが起きたとき、どのエンティティを使うか:

```
問いや不確実性が生まれた
  └→ Thread を作成して議論を始める
       ├→ 事実が判明 → Entry (finding)
       ├→ 選択肢が出た → Entry (option)
       ├→ 賛否が論じられた → Entry (argument)
       ├→ 結論に達した → Entry (conclusion)  
       └→ 方針が決まった → orbit decide → Section更新

明確な設計変更（議論不要）
  └→ orbit edit → Decision生成 + Section更新

作業が必要になった
  └→ Task 作成（source: Decision or Thread）
       └→ 完了 → Task更新（Decisionではない）

テスト結果・調査結果
  └→ Thread Entry (finding)（Decisionではない）

複数のThreadが同じテーマだと判明
  └→ Topic 作成 → Thread紐づけ

重要な時点をマークしたい
  └→ Milestone 設定
```
