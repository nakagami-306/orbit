#!/usr/bin/env python3
"""
PreToolUse hook (matcher: Bash):
orbit decide / orbit edit コマンドの実行前にバリデーション。

チェック項目:
1. orbit decide に -s (section) が指定されているか
2. titleが「〜を反映」「〜を実装」等の作業/転記表現でないか
3. rationaleが空・自己参照的・極端に短くないか
4. --content がいきなり箇条書きで始まっていないか（読者モデル違反）

exit 0: approve（通常のコマンド or バリデーション通過）
exit 2: deny（バリデーション失敗。AIにreasonを返す）
"""
import sys
import os
import json
import re

def extract_quoted(flag: str, command: str) -> str | None:
    """-X "value" / -X 'value' / --xxx "value" を抽出。"""
    patterns = [
        rf'{flag}\s+"((?:[^"\\]|\\.)*)"',
        rf"{flag}\s+'((?:[^'\\]|\\.)*)'",
    ]
    for p in patterns:
        m = re.search(p, command)
        if m:
            # シェルエスケープを軽く戻す
            return m.group(1).replace('\\"', '"').replace("\\'", "'")
    return None

def check_title(title: str) -> str | None:
    """title品質チェック。問題があればエラーメッセージを返す。"""
    bad_patterns = [
        (r'を反映$', "「〜を反映」はDecisionのtitleではありません。"),
        (r'を同期$', "「〜を同期」はDecisionのtitleではありません。"),
        (r'を更新$', "「〜を更新」はDecisionのtitleではありません。"),
        (r'を記録$', "「〜を記録」はDecisionのtitleではありません。"),
        (r'を実装$', "「〜を実装」は作業報告です。Decisionは設計判断。実装報告は orbit task update --status done を使ってください。"),
        (r'を修正$', "「〜を修正」は作業報告です。設計上の岐路で何を選んだかをtitleにしてください。"),
        (r'を対応$', "「〜を対応」は作業報告です。代替案から何を選んだかを書いてください。"),
        (r'を追加$', "「〜を追加」は作業報告です。なぜその追加なのか、選択肢の中で何を選んだかを書いてください。"),
    ]
    for pattern, msg in bad_patterns:
        if re.search(pattern, title):
            return f'title「{title}」: {msg}'
    return None

def check_rationale(rationale: str) -> str | None:
    """rationale品質チェック。問題があればエラーメッセージを返す。"""
    # 極端に短い
    if len(rationale.strip()) < 15:
        return f'rationale が短すぎます（{len(rationale.strip())}字）。「なぜAではなくBか」を15字以上で書いてください。'
    bad_patterns = [
        (r'^方針に基づき', "「方針に基づき」はrationaleではありません。なぜその方針かを書いてください。"),
        (r'^反映', "「反映」はrationaleではありません。なぜその選択をしたかを書いてください。"),
        (r'^実装(?:した|完了)', "「実装した」はrationaleではありません。なぜその設計を選んだかを書いてください。"),
        (r'^テストが通(?:った|る)', "テスト結果はrationaleではありません。orbit thread add --type finding で記録してください。"),
        (r'^動作確認', "動作確認はrationaleではありません。orbit thread add --type finding で記録してください。"),
    ]
    for pattern, msg in bad_patterns:
        if re.search(pattern, rationale):
            return msg
    return None

def check_content_reader_model(content: str) -> str | None:
    """Section content の読者モデルチェック。"""
    stripped = content.lstrip()
    if not stripped:
        return None
    first_line = stripped.split('\n', 1)[0].strip()
    # いきなり箇条書き / 番号付きリストで始まる
    if re.match(r'^[-*+]\s', first_line) or re.match(r'^\d+\.\s', first_line):
        return ('Section content が箇条書きで始まっています。'
                '読者（チャットコンテキストを共有していない人）向けに、まず1段落で「これは何の話か」を説明してから箇条書きに入ってください。')
    # 見出しから始まる場合は許容（### 等で構造化されている）
    return None

def main():
    if not os.path.isdir(".orbit"):
        sys.exit(0)

    try:
        input_data = json.loads(sys.stdin.read())
    except (json.JSONDecodeError, EOFError):
        sys.exit(0)

    command = input_data.get("tool_input", {}).get("command", "")
    if not command:
        sys.exit(0)

    if not re.search(r'\borbit\s+(decide|edit)\b', command):
        sys.exit(0)

    errors = []

    # orbit decide には -s が必須
    if re.search(r'\borbit\s+decide\b', command):
        if not re.search(r'\s-s\s', command) and not re.search(r'\s--section\s', command):
            errors.append("orbit decide には -s (section) が必須です。DecisionはSectionを更新しなければなりません。")

    title = extract_quoted('-t', command) or extract_quoted('--title', command)
    if title:
        msg = check_title(title)
        if msg:
            errors.append(msg)

    rationale = extract_quoted('-r', command) or extract_quoted('--rationale', command)
    if rationale:
        msg = check_rationale(rationale)
        if msg:
            errors.append(msg)

    content = extract_quoted('--content', command)
    if content:
        msg = check_content_reader_model(content)
        if msg:
            errors.append(msg)

    if errors:
        output = {
            "hookSpecificOutput": {
                "permissionDecision": "deny",
                "reason": (
                    "[Orbit DecisionGuard] "
                    + " / ".join(errors)
                    + " 判定: 代替案から1つを選んだか？ → Noなら orbit task update / orbit thread add を使ってください。"
                    + " Section内容は読者モデル（チャットコンテキスト無し）を意識してください。"
                )
            }
        }
        print(json.dumps(output))
        sys.exit(2)

    sys.exit(0)

if __name__ == "__main__":
    main()
