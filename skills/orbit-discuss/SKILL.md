---
name: orbit-discuss
description: >
  TRIGGER: 設計上の問い・不確実性が生まれた場合。問題の報告、仕様変更の提案、代替案の検討。
  「どうすべきか」「〜の方が良いのでは」「問題がある」「仕様変更」「議論したい」「thread」等のキーワード。
  ユーザーとの議論が始まりそうな場面で、まだThreadが立っていなければ発火する。
allowed-tools:
  - Bash
  - Read
---

# /orbit-discuss — Thread作成 → 議論 → Decision収束

## いつ使うか

**設計上の不確実性があるとき。** 正解がすぐにわからない、選択肢が複数ある、トレードオフの検討が必要 — これらはすべてThreadを立ててから議論する。「とりあえず議論してから決める」場面すべてが対象。

逆に、明確な修正（typo、既知バグ）やユーザーの直接指示は `/orbit-evolve` で直接変更する。

## ワークフロー

### Phase 1: 起票

議論を始める前にThreadを立てる。

```bash
orbit thread create "スレッドタイトル" --question "何を検討しているか（1文で）"
```

titleは後から見て何の議論かわかるように。questionは判断が必要な問いを明確に。

### Phase 2: 議論と記録

**ここが最も重要。チャットで議論しながら、その都度Entryを記録する。**

「後でまとめて」は禁止。理由: チャットのコンテキストは圧縮で消えるが、Orbitの記録は永続する。

```bash
# 事実が判明した（調査結果、コードの現状、制約など）
orbit thread add <thread-id> --type finding --content "..."

# 選択肢が出た
orbit thread add <thread-id> --type option --content "案A: ..."

# 選択肢の賛否
orbit thread add <thread-id> --type argument --target <option-entry-id> --stance for --content "..."
orbit thread add <thread-id> --type argument --target <option-entry-id> --stance against --content "..."

# 結論に達した
orbit thread add <thread-id> --type conclusion --content "..."

# 上記に当てはまらないメモ
orbit thread add <thread-id> --type note --content "..."
```

### Entry typeの判定

チャットの中で何が起きたかを見て、最も適切なtypeを選ぶ:

| チャットで起きたこと | type | 判断基準 |
|---------------------|------|---------|
| コードを読んで何かがわかった | finding | 客観的な事実の発見 |
| 「こういう方法もある」と提案された | option | 比較対象となる選択肢 |
| 「その案は〜だから良い/悪い」 | argument | 特定のoptionに対する評価。target + stance |
| 「じゃあこれでいこう」 | conclusion | 方針の確定。orbit decideへの橋渡し |
| 「これは別threadで扱おう」 | note | 上記のどれにも当てはまらない |

**迷ったらfinding。** 事実の記録は最も安全で、後から再評価できる。

### 記録の粒度

- 1ターンの議論で複数種類の情報が出た → 種類ごとに別Entryで記録
- 同じ種類が連続した → 1つのEntryにまとめてよい
- conclusionは議論の収束時のみ。途中の仮結論はnoteで
- argumentは必ずtarget（どのoptionに対するものか）とstance（for/against/neutral）を指定

### Phase 3: 収束

結論が出たら、ThreadをDecisionに収束させる。

**orbit decide前のセルフチェック（必須）:**
1. これは設計上の選択か？代替案から1つを選んだか？
2. rationaleは「なぜAではなくBか」を説明しているか？
3. Section内容は最新の設計状態を反映しているか？

```bash
# Section全文更新
orbit decide <thread-id> -s "Section名" -t "何を決めたか" -r "なぜそう決めたか" --content "Sectionの全文"

# 部分置換
orbit decide <thread-id> -s "Section名" -t "何を決めたか" -r "理由" --old "置換前" --new "置換後"
```

Decision作成とSection更新は同一タイミングで完結させる。「後で反映」は禁止。

### Phase 4: 放棄

議論の結果、対処不要と判断した場合:

```bash
orbit thread close <thread-id> --abandon
```

放棄されたThreadも履歴として残る（検討した結果やらないことにした、という記録）。

## Topic（複数Threadの紐づけ）

複数のThreadが同じテーマの別側面だと判明したら:

```bash
orbit topic create "テーマ名" --description "背景"
orbit topic add-thread <topic-id> <thread-id>
```

Topicは事後的な紐づけ。事前に計画して作るものではない。
