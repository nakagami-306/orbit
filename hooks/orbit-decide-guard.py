#!/usr/bin/env python3
"""
PreToolUse hook (matcher: Bash):
orbit decide / orbit edit コマンドの実行前にバリデーション。

チェック項目:
1. orbit decide に -s (section) が指定されているか
2. titleが「〜を反映」「〜を同期」「〜を更新」パターンではないか
3. rationaleが空や自己参照的でないか

exit 0: approve（通常のコマンド or バリデーション通過）
exit 2: deny（orbit decideのバリデーション失敗）
"""
import sys
import os
import json
import re

def main():
    if not os.path.isdir(".orbit"):
        sys.exit(0)

    # stdinからtool inputを読む
    try:
        input_data = json.loads(sys.stdin.read())
    except (json.JSONDecodeError, EOFError):
        sys.exit(0)

    # Bash tool の command を取得
    command = input_data.get("tool_input", {}).get("command", "")
    if not command:
        sys.exit(0)

    # orbit decide / orbit edit 以外はスルー
    if not re.search(r'\borbit\s+(decide|edit)\b', command):
        sys.exit(0)

    errors = []

    # orbit decide には -s が必須
    if re.search(r'\borbit\s+decide\b', command):
        if not re.search(r'\s-s\s', command) and not re.search(r'\s--section\s', command):
            errors.append("orbit decide には -s (section) が必須です。DecisionはSectionを更新しなければなりません。")

    # titleパターンのチェック（-t "..." の中身）
    title_match = re.search(r'-t\s+"([^"]+)"', command) or re.search(r"-t\s+'([^']+)'", command)
    if title_match:
        title = title_match.group(1)
        bad_patterns = [
            (r'を反映$', "「〜を反映」はDecisionのtitleではありません。何を決めたかを書いてください。"),
            (r'を同期$', "「〜を同期」はDecisionのtitleではありません。何を決めたかを書いてください。"),
            (r'を更新$', "「〜を更新」はDecisionのtitleではありません。何を決めたかを書いてください。"),
            (r'を記録$', "「〜を記録」はDecisionのtitleではありません。何を決めたかを書いてください。"),
        ]
        for pattern, msg in bad_patterns:
            if re.search(pattern, title):
                errors.append(msg)
                break

    # rationaleのチェック（-r "..." の中身）
    rationale_match = re.search(r'-r\s+"([^"]+)"', command) or re.search(r"-r\s+'([^']+)'", command)
    if rationale_match:
        rationale = rationale_match.group(1)
        bad_rationales = [
            (r'^方針に基づき', "「方針に基づき」はrationaleではありません。なぜその選択をしたかを書いてください。"),
            (r'^反映$', "「反映」はrationaleではありません。なぜその選択をしたかを書いてください。"),
        ]
        for pattern, msg in bad_rationales:
            if re.search(pattern, rationale):
                errors.append(msg)
                break

    if errors:
        output = {
            "hookSpecificOutput": {
                "permissionDecision": "deny",
                "reason": "[Orbit DecisionGuard] " + " / ".join(errors) + " Decisionは設計上の岐路で選択肢から1つを選んだ記録です。判定: 代替案から1つを選んだか？ → Noなら orbit task update / orbit thread add を使ってください。"
            }
        }
        print(json.dumps(output))
        sys.exit(2)

    sys.exit(0)

if __name__ == "__main__":
    main()
