# Orbit

**プロジェクトの「今の設計」と「なぜそうなったか」を管理するCLIツール。**

---

## こんな経験はないか

- 設計を議論して決定したのに、3ヶ月後に「なんでこの方針になったんだっけ？」と誰も答えられない
- 複数のドキュメントに設計情報が散らばっていて、**プロジェクトの今の全体像**がどこにもない
- 「前にこの案は検討して却下したはず」と思うが、却下した理由が見つからない
- 設計の代替案を試したいが、今の設計を壊さずに「もしこうだったら」を書ける場所がない
- Linear/Jiraはコードのissueには使えるが、設計のリサーチや意思決定を管理する場所がない

Orbitはこれらを解決する。

## Orbitがやること

**プロジェクトの設計状態をgitのようにバージョン管理する。**

gitがソースコードの変更履歴を管理するように、Orbitは設計ドキュメントの変更履歴を管理する。全ての変更に「何を変えたか」「なぜ変えたか」が記録され、任意の時点の設計に戻れる。

gitがブランチで並行開発するように、Orbitは設計の代替案をブランチで並行探索し、比較し、マージできる。

gitがコードだけを扱うように、Orbitはプロジェクトの種類を問わない。ソフトウェアでも、事業企画でも、投資戦略でも、設計と意思決定があるものなら何でも管理できる。

## 30秒で試す

```bash
# プロジェクトを作成
orbit init "my-project"

# 設計を書く
orbit edit -t "初期構想" -r "プロジェクトの方向性を定義" \
  --content "顧客向けヘルスケアアプリ。月額制サブスクリプション。"

# 見る
orbit show
```

これで `.orbit/state.md` にプロジェクトの設計状態が自動生成される。以降、`orbit edit` で変更するたびに、変更理由とともに履歴が積み上がっていく。

## もう少し使ってみる

```bash
# セクションに分けて整理
orbit section add "ターゲット市場" --content "25-45歳の都市部在住者"
orbit section add "収益モデル" --content "月額3,000-8,000円のサブスク"

# 設計を変更（理由付き）
orbit edit -t "ターゲット拡大" -r "市場調査の結果を反映" \
  -s "ターゲット市場" --content "20-50歳の健康意識層"

# なぜ今こうなっているか辿る
orbit section log "ターゲット市場"

# 代替案をブランチで探索
orbit branch create --name "enterprise-pivot"
orbit branch switch "enterprise-pivot"
orbit edit -t "B2B転換" -r "法人向けの方が市場が大きい" \
  -s "ターゲット市場" --content "従業員500人以上の企業の福利厚生部門"

# メインの設計はそのまま
orbit branch switch main
orbit show  # 元の設計が表示される

# 2つの案を比較
orbit diff main enterprise-pivot
```

## ドキュメント

| ドキュメント | 内容 |
|-------------|------|
| **[ガイド](docs/guide.md)** | Orbitの概念と考え方を理解する。ワークフローの具体例付き |
| **[CLIリファレンス](docs/cli-reference.md)** | 全コマンドの引数・フラグ一覧 |

## インストール

```bash
go install github.com/nakagami-306/orbit/cmd/orbit@latest
```

または:

```bash
git clone https://github.com/nakagami-306/orbit.git
cd orbit
go build ./cmd/orbit/
```

Go 1.22以上が必要。

## ライセンス

Private
