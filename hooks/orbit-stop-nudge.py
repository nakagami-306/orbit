#!/usr/bin/env python3
"""
Stop hook: assistantターン終了時に議論記録漏れの可能性を検出してnudge。

判定ロジック（すべて構造的、AI判定なし）:
1. transcript_pathから直近userメッセージ以降のassistantイベントを取得
2. assistant出力本文に議論キーワード（提案/思う/トレードオフ/論点/代替案/〜の方が/〜すべき/賛成/反対 等）が
   2件以上含まれかつ本文が400字以上
3. 同期間にBashツールでorbit thread add呼び出しが0件
→ block + reason で「議論内容を記録しましたか？」とnudge

ループ防止:
- stop_hook_active=true なら即exit 0
- 失敗時は静かにexit 0（hookで本処理を妨げない）

廃止前のAI判定型と異なり、判定基準は固定の正規表現＋件数のみ。
精度は完璧でないが、誤検知の実害は「nudgeが余計に出る」だけ。
"""
import sys
import os
import json
import re

DISCUSSION_KEYWORDS = [
    r'提案',
    r'思う',
    r'思います',
    r'と判断',
    r'トレードオフ',
    r'論点',
    r'代替案',
    r'対案',
    r'(?:の|が)方が(?:良い|よい|妥当|筋)',
    r'すべき',
    r'べきだ',
    r'賛成',
    r'反対',
    r'懸念',
    r'リスク',
    r'検討',
    r'案[A-Z]',
    r'選択肢',
    r'(?<![\w])vs(?![\w])',
    r'ではなく',
]

MIN_BODY_LEN = 400          # この字数未満のassistant turnはスキップ（軽い応答）
MIN_KEYWORD_HITS = 2        # 議論性とみなすキーワード件数の下限
ORBIT_RECORD_PATTERN = re.compile(r'\borbit\s+thread\s+add\b')

def safe_exit(code: int = 0):
    sys.exit(code)

def read_transcript(path: str) -> list[dict]:
    events = []
    try:
        with open(path, 'r', encoding='utf-8') as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    events.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
    except (OSError, IOError):
        pass
    return events

def slice_last_turn(events: list[dict]) -> list[dict]:
    """直近の user イベント以降を返す。なければ全件。"""
    last_user = -1
    for i in range(len(events) - 1, -1, -1):
        ev = events[i]
        if ev.get('type') == 'user' or ev.get('role') == 'user':
            last_user = i
            break
        msg = ev.get('message')
        if isinstance(msg, dict) and msg.get('role') == 'user':
            last_user = i
            break
    if last_user < 0:
        return events
    return events[last_user + 1:]

def extract_assistant_text(events: list[dict]) -> str:
    """assistantイベント群からテキスト本文を集約。"""
    chunks = []
    for ev in events:
        msg = ev.get('message')
        if not isinstance(msg, dict):
            if ev.get('type') == 'assistant' and isinstance(ev.get('content'), str):
                chunks.append(ev['content'])
            continue
        if msg.get('role') != 'assistant':
            continue
        content = msg.get('content')
        if isinstance(content, str):
            chunks.append(content)
        elif isinstance(content, list):
            for c in content:
                if isinstance(c, dict) and c.get('type') == 'text':
                    txt = c.get('text', '')
                    if isinstance(txt, str):
                        chunks.append(txt)
    return '\n'.join(chunks)

def extract_bash_commands(events: list[dict]) -> list[str]:
    """assistantのtool_use(Bash)コマンドを集約。"""
    commands = []
    for ev in events:
        msg = ev.get('message')
        if not isinstance(msg, dict) or msg.get('role') != 'assistant':
            continue
        content = msg.get('content')
        if not isinstance(content, list):
            continue
        for c in content:
            if not isinstance(c, dict):
                continue
            if c.get('type') != 'tool_use':
                continue
            if c.get('name') != 'Bash':
                continue
            inp = c.get('input', {})
            if isinstance(inp, dict):
                cmd = inp.get('command')
                if isinstance(cmd, str):
                    commands.append(cmd)
    return commands

def main():
    if not os.path.isdir(".orbit"):
        safe_exit(0)

    try:
        input_data = json.loads(sys.stdin.read())
    except (json.JSONDecodeError, EOFError):
        safe_exit(0)

    if input_data.get('stop_hook_active'):
        safe_exit(0)

    transcript_path = input_data.get('transcript_path')
    if not transcript_path or not os.path.isfile(transcript_path):
        safe_exit(0)

    events = read_transcript(transcript_path)
    if not events:
        safe_exit(0)

    last_turn = slice_last_turn(events)
    if not last_turn:
        safe_exit(0)

    assistant_text = extract_assistant_text(last_turn)
    if len(assistant_text) < MIN_BODY_LEN:
        safe_exit(0)

    hits = sum(1 for kw in DISCUSSION_KEYWORDS if re.search(kw, assistant_text))
    if hits < MIN_KEYWORD_HITS:
        safe_exit(0)

    bash_cmds = extract_bash_commands(last_turn)
    recorded = any(ORBIT_RECORD_PATTERN.search(c) for c in bash_cmds)
    if recorded:
        safe_exit(0)

    output = {
        "decision": "block",
        "reason": (
            "[Orbit Nudge] 直近のターンに議論的内容（提案/比較/論点等）が含まれているようですが、"
            f"orbit thread add の呼び出しが見当たりません（議論キーワード {hits} 件検出、本文 {len(assistant_text)} 字）。"
            " 議論をthreadに記録すべきならば orbit thread add で記録してから停止してください。"
            " 単なる作業報告で議論性のないターンなら、その旨を明示してそのまま続行してください。"
        )
    }
    print(json.dumps(output, ensure_ascii=False))
    sys.exit(2)

if __name__ == "__main__":
    main()
