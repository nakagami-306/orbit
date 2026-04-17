---
name: orbit-evolve
description: >
  TRIGGER: 設計状態（State/Section）の変更。明確な設計変更で議論が不要な場合。
  「設計変更」「Section更新」「orbit edit」「State変更」「方針変更」等のキーワード。
  不確実性がある場合は /orbit-discuss を使うこと。
allowed-tools:
  - Bash
  - Read
---

# /orbit-evolve — State編集 → Decision生成

## いつ使うか

**設計変更が明確で、議論が不要な場合。** ユーザーが「こう変更して」と直接指示した場合、または自明な改善の場合。

不確実性がある（選択肢が複数ある、トレードオフがある）なら、先に `/orbit-discuss` でThreadを立てて議論する。

## Decision作成前のセルフチェック（必須）

orbit edit を実行する前に、以下3つを確認する:

| チェック項目 | NGの例 | OKの例 |
|-------------|--------|--------|
| 代替案から1つを選んだか？ | テスト完了の報告 | 「user_versionではなくmigrationテーブルを採用」 |
| rationaleは「なぜ」を説明しているか？ | 「方針に基づき反映」 | 「SQLite標準で追加テーブル不要のため」 |
| titleは意思決定の内容か？ | 「〜を反映」「〜を同期」 | 「マイグレーション方式をuser_versionベースに変更」 |

**1つでもNGなら、それはDecisionではない。正しい振り分け先:**
- 実装完了 → `orbit task update <id> --status done`
- テスト結果 → `orbit thread add <thread-id> --type finding`
- 議論メモ → `orbit thread add <thread-id> --type note`

## ワークフロー

### 1. 現在のStateを確認

```bash
orbit show
```

### 2. 変更を実行

**新しいSectionの追加:**
```bash
orbit section add -t "タイトル" --content "内容" --rationale "なぜ必要か"
```

**既存Sectionの部分変更:**
```bash
orbit edit -s "Section名" -t "何を決めたか" -r "なぜ変更するか" --old "置換前" --new "置換後"
```

**既存Sectionの全文書き換え:**
```bash
orbit edit -s "Section名" -t "何を決めたか" -r "なぜ変更するか" --content "新しい全文"
```

### 3. 確認

```bash
orbit status  # stale sectionがないか確認
```

stale sectionがあれば、連鎖的にそちらも更新を検討する。

## アンチパターン（過去のAIの誤用実例）

| パターン | 何が問題か | 正しい対処 |
|---------|-----------|-----------|
| 「パッチモードの動作確認を反映」というDecision | テスト結果はDecisionではない | `orbit thread add --type finding` |
| 「方針に基づきSectionを更新」というDecision | 転記作業はDecisionではない | 元のDecision時点でSection更新を完了させる |
| Section更新のためだけにDecisionを作成 | 因果が逆転している | Decisionが先、Section更新が結果 |
| DecisionとSection更新を別タイミングで実行 | 後追い反映Decisionが発生する | 同時に完結させる |
