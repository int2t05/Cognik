"""
从 ModelScope（魔搭社区）下载轻量 cross-encoder 模型到本地目录。

用法（在 server/ 目录下执行）：
    pip install modelscope
    python models/rerank/download.py

默认模型：ms-marco-MiniLM-L-4-v2（~50MB）
切换模型：RERANK_MODEL=ms-marco-MiniLM-L-6-v2 python models/rerank/download.py
"""

import os
import sys

MODEL_NAME = os.environ.get("RERANK_MODEL", "ms-marco-MiniLM-L-4-v2")
MODELSCOPE_PATH = f"AI-ModelScope/{MODEL_NAME}"
MODEL_DIR = os.path.dirname(os.path.abspath(__file__))

# 已下载则跳过
required = ["config.json", "model.safetensors", "tokenizer_config.json", "vocab.txt"]
if all(os.path.exists(os.path.join(MODEL_DIR, f)) for f in required):
    total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
                for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
    print(f"模型已存在 ({total:.1f} MB)，跳过下载")
    sys.exit(0)

try:
    from modelscope import snapshot_download
except ImportError:
    print("请先安装: pip install modelscope")
    sys.exit(1)

print(f"下载 {MODELSCOPE_PATH} → {MODEL_DIR}")
snapshot_download(MODELSCOPE_PATH, local_dir=MODEL_DIR)

total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
            for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
print(f"完成 ({total:.1f} MB)")
