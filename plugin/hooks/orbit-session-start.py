#!/usr/bin/env python3
"""
SessionStart hook: Orbitプロジェクト検出時にエンティティ意味論・行動ルール・現在の状態を注入。

出力: hookSpecificOutput.additionalContext にテキストを載せてシステムプロンプトに追加。
.orbit/ が存在しない場合は何も出力しない。
"""
import subprocess
import os
import json
import sys

def run_orbit(args: list[str]) -> str:
    try:
        result = subprocess.run(
            ["orbit"] + args,
            capture_output=True, text=True, encoding="utf-8", timeout=10
        )
        return result.stdout.strip() if result.returncode == 0 else ""
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return ""

def main():
    if not os.path.isdir(".orbit"):
        return

    status = run_orbit(["status"])
    if not status:
        return

    thread_list = run_orbit(["thread", "list"])
    open_threads = []
    if thread_list:
        for line in thread_list.strip().split("\n"):
            if "[open]" in line:
                open_threads.append(line.strip())

    ctx = []
    ctx.append("[Orbit] このプロジェクトはOrbitで管理されている。以下はセッション全体を通じて適用される。")
    ctx.append("")

    # --- エンティティ意味論 ---
    ctx.append("# Orbit エンティティ意味論")
    ctx.append("")
    ctx.append("## Decision — 設計変更の原子単位")
    ctx.append("設計上の岐路で選択肢から1つを選んだ記録。title=何を決めたか、rationale=なぜそう決めたか。")
    ctx.append("Decisionであるもの: 技術選定、設計方針の確定、アーキテクチャの変更")
    ctx.append("Decisionでないもの: 実装完了報告、テスト結果、既存内容の転記、作業ログ")
    ctx.append("判定: 「代替案から1つを選んだか？」→ No ならDecisionではない")
    ctx.append("必須制約: 全DecisionはいずれかのSectionを更新する。Decision作成とSection更新は同時に完結させる")
    ctx.append("")
    ctx.append("## Section — Stateの区画")
    ctx.append("プロジェクトの設計全体像（State）の名前付き区画。常に最新状態のスナップショット。")
    ctx.append("累積的な追記ドキュメントではない。過去の状態はDecision DAGが担う。")
    ctx.append("")
    ctx.append("## Thread — 議論・検討の場")
    ctx.append("1つの問いに対する構造化された議論の場。status: open → decided / abandoned")
    ctx.append("作るべきとき: 不確実性がある、選択肢が複数ある、トレードオフがある")
    ctx.append("")
    ctx.append("## Entry — Threadの構造化記録単位")
    ctx.append("finding=調査で判明した事実, option=選択肢, argument=特定optionへの賛否(target+stance), conclusion=結論, note=その他")
    ctx.append("迷ったらfinding。議論の中で該当する発言が出たらその都度すぐ記録する。後でまとめない。")
    ctx.append("")
    ctx.append("## Topic — Thread群のテーマ単位")
    ctx.append("関連Thread群を事後的に紐づけるテーマ。事前の計画ではなく「同じ話だった」と判明した時点で作成。")
    ctx.append("")
    ctx.append("## Task — 実行可能なアクション")
    ctx.append("やること。source: Decision由来（設計を実行）/ Thread由来（調査・作業）/ 独立。")
    ctx.append("実装完了 = orbit task update --status done。Decisionではない。")
    ctx.append("")
    ctx.append("## Milestone — Decision DAGのマーカー")
    ctx.append("特定Decision時点の名前付きポインタ。時間旅行の基準点。")
    ctx.append("")

    # --- 行動振り分け ---
    ctx.append("# 行動振り分け")
    ctx.append("")
    ctx.append("問い・不確実性 → orbit thread create → 議論中はorbit thread addでその都度記録")
    ctx.append("方針決定 → orbit decide（Decision + Section更新を同時に完結）")
    ctx.append("明確な設計変更（議論不要） → orbit edit")
    ctx.append("作業が必要 → orbit task create")
    ctx.append("実装完了 → orbit task update --status done（Decisionではない）")
    ctx.append("テスト結果・調査結果 → orbit thread add --type finding（Decisionではない）")
    ctx.append("複数Threadが同一テーマ → orbit topic create + add-thread")
    ctx.append("")

    # --- Decisionガードレール ---
    ctx.append("# Decisionガードレール")
    ctx.append("")
    ctx.append("orbit decide / orbit edit の前に必ず確認:")
    ctx.append("1. 代替案から1つを選んだか？ → No ならDecisionではない")
    ctx.append("2. rationaleは「なぜAではなくBか」か？ → 「反映した」は理由ではない")
    ctx.append("3. titleは意思決定の内容か？ → 「〜を反映」「〜を同期」はNG")
    ctx.append("")
    ctx.append("過去のAI誤用例（やってはいけない）:")
    ctx.append("- 「パッチモードの動作確認を反映」← テスト結果はfinding")
    ctx.append("- 「方針に基づきSectionを更新」← 転記はDecisionではない")
    ctx.append("- Decision作成とSection更新を別タイミング ← 同時に完結させる")
    ctx.append("")

    # --- 議論記録ルール ---
    ctx.append("# 議論記録ルール")
    ctx.append("")
    ctx.append("チャット上の議論はorbit thread addでその都度記録する。「後でまとめて」は禁止。")
    ctx.append("チャットのコンテキストは圧縮で消えるが、Orbitの記録は永続する。")
    ctx.append("Stop hookが議論の記録漏れを検知してブロックすることがある。その場合は記録してから続行する。")
    ctx.append("")

    # --- 現在の状態 ---
    ctx.append("# 現在の状態")
    ctx.append("")
    ctx.append(status)
    ctx.append("")

    if open_threads:
        ctx.append("## Open Threads")
        for t in open_threads:
            ctx.append(f"  {t}")
        ctx.append("")

    output = {
        "hookSpecificOutput": {
            "hookEventName": "SessionStart",
            "additionalContext": "\n".join(ctx)
        }
    }
    print(json.dumps(output))

if __name__ == "__main__":
    main()
