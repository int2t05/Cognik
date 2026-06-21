"""
Cross-encoder 重排序推理服务（常驻子进程，stdin/stdout JSON Lines 协议）。

用途：供 Go 侧 SubprocessReranker 通过 os/exec 启动，替换 LLM prompt 重排序。
模型：cross-encoder/ms-marco-MiniLM-L-4-v2（~80MB），轻量 cross-encoder。

通信协议（JSON Lines）：
  输入（stdin） → {"query": "...", "passages": [{"id": 0, "text": "..."}, ...]}
  输出（stdout）→ {"order": [2, 0, 1], "scores": [0.92, 0.45, 0.12]}
  错误输出（stdout）→ {"error": "message"}

环境变量：
  RERANK_MODEL      模型名或本地路径，默认 cross-encoder/ms-marco-MiniLM-L-4-v2
  RERANK_DEVICE     推理设备，默认 cpu（可用 cuda / mps）
"""

from __future__ import annotations

import json
import os
import sys
import signal
import logging
from typing import Any, Dict, List

import torch
from sentence_transformers import CrossEncoder

# ── 日志：输出到 stderr（避免污染 stdout JSON Lines 协议） ──
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [rerank] %(levelname)s %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger("rerank_server")

# ── 模型加载 ──
# 优先使用环境变量，其次本地模型目录（预下载），最后 ModelScope 在线加载
_LOCAL_MODEL = os.path.join(os.path.dirname(os.path.abspath(__file__)), "models", "rerank")
_DEFAULT_MODEL = "cross-encoder/ms-marco-MiniLM-L-4-v2"
if os.path.exists(os.path.join(_LOCAL_MODEL, "model.safetensors")):
    _DEFAULT_MODEL = _LOCAL_MODEL
# 优先国内镜像，加速在线下载
os.environ.setdefault("HF_ENDPOINT", "https://hf-mirror.com")
MODEL_NAME = os.environ.get("RERANK_MODEL", _DEFAULT_MODEL)
DEVICE = os.environ.get("RERANK_DEVICE", "cpu")

logger.info("加载 Cross-Encoder 模型: %s (device=%s)", MODEL_NAME, DEVICE)

# FP16 以减少内存占用（~560MB vs ~1.1GB FP32）
model = CrossEncoder(
    MODEL_NAME,
    device=DEVICE,
    default_activation_function=torch.nn.Sigmoid(),
)
if DEVICE == "cpu":
    try:
        model.model.half()  # 原地转换，不重新赋值
        logger.info("模型已转为 FP16")
    except Exception:
        logger.info("模型 FP16 转换失败，保持默认精度")

logger.info("模型加载完成，等待输入...")


def write_result(data: Dict[str, Any]) -> None:
    """写 JSON 行到 stdout 并立即 flush，确保 Go 侧能及时读取。"""
    sys.stdout.write(json.dumps(data, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def handle_request(line: str) -> None:
    """处理单条 JSON 请求。"""
    try:
        req = json.loads(line)
    except json.JSONDecodeError as e:
        write_result({"req_id": "", "error": f"JSON 解析失败: {e}"})
        return

    req_id = req.get("req_id", "")
    query = req.get("query", "")
    passages = req.get("passages", [])

    if not query or not passages:
        write_result({"req_id": req_id, "order": [], "scores": [], "error": "query 或 passages 为空"})
        return

    # 构建 (query, passage) 对
    pairs = [(query, p.get("text", "")) for p in passages]
    ids = [p.get("id", i) for i, p in enumerate(passages)]

    # Cross-encoder 批量打分
    try:
        scores = model.predict(pairs, show_progress_bar=False)
    except Exception as e:
        logger.error("模型推理失败: %s", e)
        write_result({"req_id": req_id, "error": f"模型推理失败: {e}"})
        return

    # 确保 scores 是一维列表
    if hasattr(scores, "tolist"):
        scores = scores.tolist()
    elif not isinstance(scores, list):
        scores = [float(scores)]

    # 按分数降序排列
    indexed = list(zip(ids, scores))
    indexed.sort(key=lambda x: x[1], reverse=True)

    order = [i for i, _ in indexed]
    sorted_scores = [s for _, s in indexed]

    write_result({"req_id": req_id, "order": order, "scores": sorted_scores})


def main() -> None:
    """主循环：读取 stdin 逐行处理。"""
    # 优雅退出
    running = True

    def on_signal(signum, frame):
        nonlocal running
        logger.info("收到信号 %s，正在退出...", signum)
        running = False

    signal.signal(signal.SIGTERM, on_signal)
    signal.signal(signal.SIGINT, on_signal)

    for line in sys.stdin:
        if not running:
            break
        line = line.strip()
        if not line:
            continue
        handle_request(line)

    logger.info("rerank_server 已退出")


if __name__ == "__main__":
    main()
