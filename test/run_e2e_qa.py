#!/usr/bin/env python3
"""
Cognik 端到端 QA 测试脚本

流程: 登录 → 创建 KB → 上传文档 → 发布 → 问答 → 记录 SQLite trace → 输出报告

用法:
    cd test
    python run_e2e_qa.py --base-url http://localhost:8080
"""

import argparse
import json
import os
import sys
import time
import requests
from pathlib import Path

# ── 配置 ──────────────────────────────────────────────

DOCS_DIR = Path(__file__).parent / "docs"
QUESTIONS_FILE = Path(__file__).parent / "questions.json"
REPORT_FILE = Path(__file__).parent / "qa-result.md"

ADMIN_USER = "admin"
ADMIN_PASS = "Admin@123"
KB_NAME = "QA-OpsKB"


def login(base_url):
    """登录获取 access_token。"""
    resp = requests.post(f"{base_url}/api/v1/auth/login",
                        json={"username": ADMIN_USER, "password": ADMIN_PASS}, timeout=10)
    data = resp.json()
    if data.get("code") != 0:
        raise RuntimeError(f"登录失败: {data.get('message')}")
    return data["data"]["access_token"]


def create_kb(base_url, token):
    """创建知识库(若已存在则复用)。"""
    headers = {"Authorization": f"Bearer {token}"}
    # 先查是否已存在
    resp = requests.get(f"{base_url}/api/v1/admin/knowledge-bases", headers=headers, timeout=10)
    for kb in resp.json()["data"]:
        if kb["name"] == KB_NAME:
            print(f"  KB 已存在,复用 id={kb['id']}")
            return kb["id"]
    # 不存在则创建
    resp = requests.post(f"{base_url}/api/v1/admin/knowledge-bases",
                        headers=headers, json={"name": KB_NAME}, timeout=10)
    data = resp.json()
    if data.get("code") != 0:
        raise RuntimeError(f"创建 KB 失败: {data.get('message')}")
    for kb in requests.get(f"{base_url}/api/v1/admin/knowledge-bases", headers=headers, timeout=10).json()["data"]:
        if kb["name"] == KB_NAME:
            return kb["id"]
    raise RuntimeError("未找到创建的 KB")


def upload_docs(base_url, token, kb_id):
    """上传 docs/ 下所有 .md 文件,返回 filename→article_id 映射。"""
    files_paths = sorted(DOCS_DIR.glob("*.md"))
    if not files_paths:
        raise RuntimeError(f"docs/ 下无 .md 文件")

    article_map = {}
    # 查已有文章(若 KB 已有文章则跳过上传)
    resp = requests.get(f"{base_url}/api/v1/admin/knowledge-bases/{kb_id}/articles",
                       headers={"Authorization": f"Bearer {token}"}, params={"page":1,"page_size":100}, timeout=10)
    existing = {}
    rdata = resp.json().get("data", {})
    art_list = rdata if isinstance(rdata, list) else rdata.get("articles", [])
    for a in art_list:
        existing[a.get("title","")] = a["id"]

    for fp in files_paths:
        if fp.stem in existing:
            article_map[fp.name] = existing[fp.stem]
            print(f"  已存在 {fp.name} → article_id={existing[fp.stem]}")
            continue
        with open(fp, "rb") as f:
            resp = requests.post(
                f"{base_url}/api/v1/admin/knowledge-bases/{kb_id}/documents/upload",
                headers={"Authorization": f"Bearer {token}"},
                files={"files": (fp.name, f, "text/markdown")},
                timeout=30,
            )
        data = resp.json()
        for doc in data["data"]["documents"]:
            if doc["success"]:
                article_map[fp.name] = doc["article_id"]
                print(f"  上传 {fp.name} → article_id={doc['article_id']}")
            else:
                print(f"  上传 {fp.name} 失败: {doc.get('error_msg', 'unknown')}")
    return article_map


def publish_article(base_url, token, article_id):
    """发布文章:submit-review → review(approved) → publish。"""
    headers = {"Authorization": f"Bearer {token}"}

    # 查当前状态,已发布的跳过
    r = requests.get(f"{base_url}/api/v1/admin/articles/{article_id}", headers=headers, timeout=10)
    cur_status = r.json().get("data", {}).get("status", 0)
    if cur_status == 4:
        print(f"  已发布(article_id={article_id})")
        return True

    # submit-review: Draft → Reviewing
    r = requests.post(f"{base_url}/api/v1/admin/articles/{article_id}/submit-review",
                     headers=headers, json={}, timeout=10)
    if r.json().get("code") != 0:
        print(f"  submit-review 失败(article_id={article_id}): {r.json().get('message')}")
        return False

    # review: Reviewing → Approved
    r = requests.post(f"{base_url}/api/v1/admin/articles/{article_id}/review",
                      headers=headers, json={"approved": True}, timeout=10)
    if r.json().get("code") != 0:
        print(f"  review 失败(article_id={article_id}): {r.json().get('message')}")
        return False

    # publish: Approved → Published (同步触发 chunk+embed+pgvector,需较长 timeout)
    r = requests.post(f"{base_url}/api/v1/admin/articles/{article_id}/publish",
                      headers=headers, json={}, timeout=120)
    if r.json().get("code") != 0:
        print(f"  publish 失败(article_id={article_id}): {r.json().get('message')}")
        return False

    return True


def wait_for_index(base_url, token, article_ids, timeout=300):
    """轮询文章状态,等待索引就绪(chunks 写入完成)。"""
    headers = {"Authorization": f"Bearer {token}"}
    deadline = time.time() + timeout
    while time.time() < deadline:
        all_ready = True
        for aid in article_ids:
            r = requests.get(f"{base_url}/api/v1/admin/articles/{aid}", headers=headers, timeout=10)
            data = r.json().get("data", {})
            chunk_count = data.get("chunk_count", 0)
            status = data.get("status", 0)
            if chunk_count == 0 or status != 4:
                all_ready = False
        if all_ready:
            print(f"  索引就绪({len(article_ids)} 篇)")
            return True
        time.sleep(5)
    print(f"  索引等待超时(部分未就绪)")
    return False


def ask_question(base_url, token, question):
    """发送问题,解析 SSE 流,返回事件列表 + 最终答案。"""
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    # 创建 thread
    r = requests.post(f"{base_url}/api/v1/portal/threads",
                     headers=headers, json={"title": question[:50]}, timeout=10)
    thread_id = r.json()["data"]["id"]

    # 发送问题(SSE 流)
    resp = requests.post(f"{base_url}/api/v1/portal/threads/{thread_id}/stream",
                        headers=headers,
                        json={"question": question},
                        stream=True, timeout=300)

    events = []
    answer = ""
    for line in resp.iter_lines(decode_unicode=True):
        if not line or not line.startswith("data: "):
            continue
        payload = line[6:]
        try:
            evt = json.loads(payload)
        except json.JSONDecodeError:
            continue
        evt_type = evt.get("type", "")
        events.append(evt)
        if evt_type == "token":
            answer += evt.get("content", "")
        elif evt_type == "done":
            answer = evt.get("metadata", {}).get("answer", answer)

    return thread_id, events, answer


def get_trace(base_url, token, thread_id):
    """查询 thread 的 SQLite trace(messages.parts)。"""
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.get(f"{base_url}/api/v1/portal/threads/{thread_id}", headers=headers, timeout=10)
    data = r.json().get("data", {})
    messages = data.get("messages", [])
    trace_parts = []
    for msg in messages:
        if msg["role"] == "assistant":
            raw = msg.get("parts", "[]")
            if not raw:
                continue
            try:
                parts = json.loads(raw)
            except (json.JSONDecodeError, TypeError):
                continue
            trace_parts.extend(parts)
    return trace_parts


def main():
    parser = argparse.ArgumentParser(description="Cognik 端到端 QA 测试")
    parser.add_argument("--base-url", default="http://localhost:8080")
    args = parser.parse_args()

    print("=== Cognik 端到端 QA 测试 ===")
    print(f"Base URL: {args.base_url}")

    # 1. 登录
    print("\n[1] 登录...")
    token = login(args.base_url)
    print(f"  token 获取成功")

    # 2. 创建 KB
    print("\n[2] 创建知识库...")
    kb_id = create_kb(args.base_url, token)
    print(f"  KB id={kb_id}")

    # 3. 上传文档
    print("\n[3] 上传文档...")
    article_map = upload_docs(args.base_url, token, kb_id)

    # 4. 发布
    print("\n[4] 发布文章(触发索引)...")
    article_ids = list(article_map.values())
    for fname, aid in article_map.items():
        print(f"  发布 {fname} (id={aid})...")
        publish_article(args.base_url, token, aid)

    # 5. 等待索引
    print("\n[5] 等待索引就绪...")
    wait_for_index(args.base_url, token, article_ids)

    # 6. 问答 + 记录 trace
    print("\n[6] 执行 QA 问答...")
    with open(QUESTIONS_FILE, "r", encoding="utf-8") as f:
        questions = json.load(f)

    results = []
    for q in questions:
        print(f"  Q{q['id']}: {q['question'][:60]}...")
        thread_id, events, answer = ask_question(args.base_url, token, q["question"])
        trace_parts = get_trace(args.base_url, token, thread_id)

        # 统计事件
        reasoning_count = sum(1 for e in events if e.get("type") == "reasoning")
        tool_calls = [e for e in events if e.get("type") == "tool_call"]
        tool_results = [e for e in events if e.get("type") == "tool_result"]

        # 提取 verdict(从 tool_result 中的 [sufficiency: ...] )
        verdict = ""
        for e in events:
            if e.get("type") == "tool_result":
                content = e.get("content", "")
                if "[sufficiency:" in content:
                    start = content.index("[sufficiency:") + len("[sufficiency:")
                    end = content.index("]", start)
                    verdict = content[start:end].strip()
                    break

        # 提取 top-1 source(从第一个 tool_result)
        top_source = ""
        for e in tool_results:
            content = e.get("content", "")
            if "来源:" in content:
                start = content.index("来源:") + len("来源:")
                top_source = content[start:start+60].strip()
                break

        results.append({
            "id": q["id"],
            "question": q["question"],
            "difficulty": q["difficulty"],
            "relevant_docs": q["relevant_docs"],
            "verdict": verdict,
            "tool_call_count": len(tool_calls),
            "reasoning_count": reasoning_count,
            "answer_length": len(answer),
            "top_source": top_source,
            "answer": answer[:200],
        })

    # 7. 输出报告
    print("\n[7] 生成报告...")
    write_report(results)
    print(f"  报告已写入 {REPORT_FILE}")


def write_report(results):
    """生成 Markdown 报告。"""
    lines = []
    lines.append("# Cognik 端到端 QA 测试报告\n")
    lines.append(f"**日期**: {time.strftime('%Y-%m-%d')}")
    lines.append(f"**KB**: QA-OpsKB (6 文档, {len(results)} 查询)")
    lines.append(f"**评估对象**: Cognik RAG 检索管道 (向量 + BM25 + RRF + CRAG)\n")

    lines.append("## 问答结果\n")
    lines.append("| # | 难度 | Question | Verdict | 工具调用数 | 回答长度 |")
    lines.append("|---|------|----------|---------|-----------|---------|")
    for r in results:
        lines.append(f"| {r['id']} | {r['difficulty']} | {r['question'][:40]} | {r['verdict']} | {r['tool_call_count']} | {r['answer_length']} |")

    lines.append("\n## Agent Trace 摘要\n")
    lines.append("| # | Question | reasoning | tool_calls | tool_results | answer 长度 |")
    lines.append("|---|----------|-----------|------------|-------------|------------|")
    for r in results:
        lines.append(f"| {r['id']} | {r['question'][:30]} | {r['reasoning_count']} | {r['tool_call_count']} | {r['tool_call_count']} | {r['answer_length']} |")

    lines.append("\n## 失败分析\n")
    weak_results = [r for r in results if not r["verdict"] or "weak" in r["verdict"]]
    if weak_results:
        for r in weak_results:
            lines.append(f"- Q{r['id']} ({r['difficulty']}): {r['question'][:50]}")
            lines.append(f"  - Verdict: {r['verdict'] or '(无)'}")
            lines.append(f"  - 相关文档: {r['relevant_docs']}")
            lines.append(f"  - Top source: {r['top_source'] or '(无)'}")
    else:
        lines.append("(无 weak verdict)")

    lines.append("\n## 环境\n")
    lines.append("- Embedding: DashScope text-embedding-v2, 1536 维")
    lines.append("- pgvector: halfvec(1536), HNSW m=16/ef_construction=200/ef_search=100")
    lines.append("- Chunker: Markdown-aware, 500 字符 / 重叠 100")
    lines.append("- CRAG: ThresholdEvaluator (0.40/0.70)")

    with open(REPORT_FILE, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))


if __name__ == "__main__":
    main()
