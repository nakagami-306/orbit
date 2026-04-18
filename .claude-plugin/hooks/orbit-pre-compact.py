#!/usr/bin/env python3
"""
PreCompact hook: コンテキスト圧縮前の最後の救出 + 行動ルール再注入。

1Mコンテキストでは滅多に発火しないが、発火時は緊急:
- 圧縮でSessionStartの注入内容が消える可能性がある → 行動ルール再注入
- チャット上の未記録の議論が失われる → rescue催促
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

    thread_list = run_orbit(["thread", "list"])
    open_threads = []
    if thread_list:
        for line in thread_list.strip().split("\n"):
            if "[open]" in line:
                open_threads.append(line.strip())

    status = run_orbit(["status"])

    ctx = []
    ctx.append("[Orbit PreCompact] コンテキスト圧縮が実行されます。")
    ctx.append("")

    if open_threads:
        ctx.append("## 未記録の議論を確認してください")
        ctx.append("Open threads:")
        for t in open_threads:
            ctx.append(f"  {t}")
        ctx.append("")
        ctx.append("このセッションで議論した内容で、まだ orbit thread add に記録していないものがあれば")
        ctx.append("圧縮前に記録してください。圧縮後はチャットの文脈が失われます。")
        ctx.append("記録すべき内容: finding(事実), option(選択肢), argument(賛否), conclusion(結論)")
        ctx.append("")

    # 行動ルール再注入（SessionStartの内容が圧縮で消える場合の保険）
    ctx.append("## 行動ルール（再掲）")
    ctx.append("- 問い・不確実性 → orbit thread create → 議論中はorbit thread addでその都度記録")
    ctx.append("- 方針決定 → orbit decide（代替案から1つを選んだか？rationaleは理由か？）+ Section更新を同時に完結")
    ctx.append("- 実装完了 → orbit task update（Decisionではない）")
    ctx.append("- テスト結果 → orbit thread add --type finding（Decisionではない）")
    ctx.append("- 「〜を反映」「〜を同期」はDecisionのtitleではない")

    if status:
        ctx.append("")
        ctx.append("## 現在の状態")
        ctx.append(status)

    output = {
        "hookSpecificOutput": {
            "hookEventName": "PreCompact",
            "additionalContext": "\n".join(ctx)
        }
    }
    print(json.dumps(output))

if __name__ == "__main__":
    main()
