# Orbit ガイド

このドキュメントでは、Orbitの考え方と使い方を説明する。コマンドの詳細な引数・フラグについては[CLIリファレンス](cli-reference.md)を参照。

---

## Orbitが解決する問題

プロジェクトの仕事の大半は、コードを書くことではない。設計を考え、調査し、選択肢を比較し、意思決定する——この過程に最も時間を使う。しかし、この過程を管理するツールがない。

**Linear/Jira** は優れたツールだが、Issue→PR→Deployというコード中心のワークフローに最適化されている。「認証方式をJWTにするかSessionにするか検討中」はIssueではない。「ターゲット市場を再定義した」はPull Requestではない。

**Notion/Confluence** にドキュメントを書くことはできるが、「今の設計はどうなっているか」と「なぜそうなったか」がドキュメントの中でごちゃ混ぜになる。3ヶ月前の議論の結論がどこに書いてあるか探すのに30分かかる。

**Orbitはこの隙間を埋める。** プロジェクトの設計状態を1つの場所で管理し、全ての変更に理由を記録し、任意の時点に戻れるようにする。

---

## 基本概念

### Project

Orbitの管理単位。1つの取り組みに1つのProject。ソフトウェアプロジェクトに限らず、事業企画、投資戦略、研究テーマなど、何でもよい。

```bash
orbit init "my-saas"
orbit init "investment-strategy"
orbit init "thesis-research"
```

全プロジェクトは1つの中央データベース（`~/.orbit/orbit.db`）で管理される。どのディレクトリにいても `orbit project list` で全プロジェクトが見える。

### State

プロジェクトの「今の設計全体像」。**1つのドキュメントとして** `orbit show` で表示される。

Stateは「今こうなっている」という事実だけを書く。「なぜこうなったか」は後述のDecisionが担う。

```
$ orbit show

# my-saas

## ターゲット市場

20-50歳の健康意識層。一次ターゲットは健康診断で要注意を受けた25-35歳。

## 収益モデル

月額3,000-8,000円のサブスクリプション。法人向けプランは従業員数課金。

## 技術方針

モノリスでMVP構築。ユーザー数1万超でマイクロサービス移行を検討。
```

### Section

Stateの中の区画。プロジェクトの初期は分割せず1つのテキストとして書き始められる。設計が複雑になったら `orbit section add` でセクションに分ける。

Sectionは他のSectionへの**依存関係**を宣言できる。「収益モデル」が「ターゲット市場」に依存している場合、ターゲット市場が変更されると収益モデルに「要確認」フラグが立つ。更新漏れを防ぐ仕組み。

### Decision — 変更とその理由

**全てのState変更はDecisionとして記録される。** これがOrbitの最も重要な概念。

```bash
orbit edit -t "ターゲット拡大" -r "市場調査で20代前半にもニーズを確認" \
  -s "ターゲット市場" --content "20-50歳の健康意識層..."
```

`-t`（タイトル：何を変えたか）と `-r`（理由：なぜ変えたか）は必須。これにより、全ての変更に理由が紐づく。

gitのcommitに相当するが、commitメッセージが「何を変えたか」に留まりがちなのに対し、Decisionは「なぜ変えたか」の記録を構造的に強制する。

#### 履歴を辿る

```bash
# Decisionの時系列一覧
orbit decision log

# 特定のDecisionの詳細（何が変わったか + 理由）
orbit decision show 019d8dab

# あるセクションを変更した全Decisionの履歴
orbit section log "ターゲット市場"
```

`orbit section log` は特に強力。「なぜターゲット市場が今こうなっているのか」を、変更の連鎖として辿れる。

#### 巻き戻す

```bash
orbit revert 019d8dab -r "市場調査データに誤りがあったため"
```

Decisionを巻き戻すと、そのDecisionで行われた全ての変更が打ち消される。元のDecisionは履歴に残り、「一度こう変更したが、後に取り消した」という経緯が追跡できる。

---

## 議論と意思決定

設計変更の前には議論がある。Orbitはこの議論を構造化して記録する。

### Thread — 議論の場

Threadは「ある問いについて考える場」。検討テーマと問いを持ち、議論を経てDecisionに収束するか、放棄される。

```bash
orbit thread create "認証方式の選定" -q "JWTかSession認証か？"
```

### Entry — 構造化された議論の単位

Threadの中には5種類のEntryを記録できる:

| 種類 | 用途 | 例 |
|------|------|-----|
| **finding** | 調査で判明した事実 | 「JWTはステートレスでスケーラブル」 |
| **option** | 検討中の選択肢 | 「案A: JWT」「案B: Session」 |
| **argument** | 選択肢への賛否 | 「JWTはSPAとの相性が良い（for）」 |
| **conclusion** | 結論 | 「JWTを採用する」 |
| **note** | その他のメモ | 自由記述 |

```bash
# 事実を記録
orbit thread add THREAD_ID --type finding \
  --content "JWTはトークン失効にサーバー側の仕組みが必要"

# 選択肢を列挙
orbit thread add THREAD_ID --type option --content "JWT: ステートレス"
orbit thread add THREAD_ID --type option --content "Session: サーバー管理"

# 選択肢への賛否（どのoptionに対してか + for/against を指定）
orbit thread add THREAD_ID --type argument \
  --target OPTION_ENTRY_ID --stance for \
  --content "SPAとの相性が良く、マイクロサービス化時にも有利"
```

### decide — 議論をDecisionに収束させる

議論がまとまったら、ThreadをDecisionに収束させてStateを更新する:

```bash
orbit decide THREAD_ID \
  -t "JWT採用" -r "SPAとの相性とスケーラビリティを重視" \
  -s "認証方式" --content "JWT + リフレッシュトークン。トークン有効期限15分。"
```

これにより:
1. Threadのステータスが「decided」になる
2. Decisionが作成される
3. 指定Sectionの内容が更新される
4. 「この議論からこの決定が生まれた」というリンクが残る

---

## 設計の並行探索

「この方向でいいのか？」と思ったとき、今の設計を壊さずに別の案を試したい。gitのブランチと同じ発想。

### ブランチの作成と切り替え

```bash
# 現在の設計状態から分岐
orbit branch create --name "microservice-idea"

# 分岐先に切り替え
orbit branch switch "microservice-idea"

# ここでの変更はmainに影響しない
orbit edit -t "マイクロサービス化" -r "スケーラビリティ検証" \
  -s "技術方針" --content "API Gateway + 3サービス構成"

# mainに戻ると元の設計のまま
orbit branch switch main
orbit show  # 「モノリスでMVP構築」のまま
```

ブランチは名前なしでも作れる（匿名ブランチ）。探索の初期段階では名前が決まらないことが多いため、後から `orbit branch name` で名前をつけられる。

### 比較

```bash
orbit diff main microservice-idea
```

2つのブランチ（あるいは任意の2つの時点）のState差分をセクション単位で表示する。

### マージ

探索した案を採用する場合、mainにマージする:

```bash
orbit branch merge microservice-idea
```

同じセクションが両方のブランチで変更されていた場合、**コンフリクト**として記録される。コンフリクトは作業を止めない——記録されたまま先に進め、情報が揃った時点で解決する。

```bash
orbit conflict list                    # 未解決の一覧
orbit conflict show CONFLICT_ID        # 詳細（各ブランチの内容を並べて表示）
orbit conflict resolve CONFLICT_ID \
  --content "解決後の内容" -r "両案を統合"
```

### 放棄

「この方向はダメだった」とわかったら放棄する。履歴は残る。「なぜこの案を試し、なぜダメだったか」が後から辿れる。

```bash
orbit branch abandon "microservice-idea" -r "現時点ではモノリスで十分"
```

---

## タスクとマイルストーン

### Task

決まったことを実行に移すためのアクション。実行主体は問わない——コード、物理的な作業、手続き、AIによる処理、何でもよい。

```bash
orbit task create "JWT認証モジュールの実装" --priority h --assignee "tanaka"
orbit task create "利用規約の法務レビュー" --priority m
orbit task list
orbit task update TASK_ID --status done
```

### Milestone

プロジェクトの重要な区切りに名前をつける。後から「MVP仕様確定の時点で設計はどうだったか」を振り返れる。

```bash
orbit milestone set "MVP仕様確定"

# 後から振り返る
orbit show --at "MVP仕様確定"   # その時点のState全体
```

---

## 横断的な管理

Orbitは全プロジェクトを1つのデータベースで管理する。プロジェクトをまたいだ活動を1つのコマンドで確認できる。

```bash
# 全プロジェクトの最近の活動
orbit log

# プロジェクト一覧
orbit project list
```

---

## AIとの協業

Orbitは人間もAIも同じCLIを使う設計になっている。

全コマンドが `--format json` でJSON出力に対応し、`--content` でエディタを介さずテキストを渡せる。AIエージェント（Claude Code等）がOrbitのCLIを直接呼び出してプロジェクト管理を行える。

```bash
# AIが使うとき
orbit show --format json                    # JSONで状態を取得
orbit edit -t "..." -r "..." --content "..."  # テキスト直渡しで編集
```

人間がAIに「プロジェクトの設計状態を更新しておいて」と指示すると、AIが `orbit show` で現状を読み、`orbit edit` で更新し、`orbit status` で結果を確認する——人間と同じワークフローを同じツールで実行する。

---

## データの保存場所

| 場所 | 役割 |
|------|------|
| `~/.orbit/orbit.db` | 中央データベース。全プロジェクトのデータが入る |
| `.orbit/config.toml` | ディレクトリとプロジェクトの紐づけ |
| `.orbit/state.md` | 現在のStateの自動生成Markdown。**読み取り専用** |

`.orbit/state.md` はorbit CLIが書き込みのたびに自動で再生成する。人間がエディタで開いて設計全体を確認したり、AIがファイルとして読み込むために存在する。直接編集してはいけない（変更理由が記録されないため）。

---

## 次のステップ

- 全コマンドの詳細 → [CLIリファレンス](cli-reference.md)
