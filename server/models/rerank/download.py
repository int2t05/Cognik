"""
下载轻量 cross-encoder 模型到本地目录，供 Docker 构建时 COPY 进镜像。

用法（在 server/ 目录下执行）：
    python models/rerank/download.py

默认模型：cross-encoder/ms-marco-MiniLM-L-4-v2（~50MB）
如需更换模型，设置环境变量 RERANK_MODEL：
    RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-6-v2 python models/rerank/download.py

内置模型对比：
    MiniLM-L-2-v2  ~17MB  2 层 transformer，最小
    MiniLM-L-4-v2  ~50MB  4 层 transformer（默认）
    MiniLM-L-6-v2  ~80MB  6 层 transformer
    MiniLM-L-12-v2 ~120MB 12 层 transformer，效果最好
"""

import os
import shutil
import sys

MODEL_NAME = os.environ.get("RERANK_MODEL", "cross-encoder/ms-marco-MiniLM-L-4-v2")
MODEL_DIR = os.path.dirname(os.path.abspath(__file__))

print(f"下载模型: {MODEL_NAME} → {MODEL_DIR}")

try:
    from sentence_transformers import CrossEncoder
except ImportError:
    print("请先安装 sentence-transformers: pip install sentence-transformers")
    sys.exit(1)

# 下载模型到 HuggingFace 缓存
print("正在从 HuggingFace 下载...")
model = CrossEncoder(MODEL_NAME, device="cpu")
print("下载完成")

# 复制模型文件到本地目录
import huggingface_hub
cache_dir = huggingface_hub.constants.HF_HUB_CACHE
if not os.path.isdir(cache_dir):
    cache_dir = os.path.expanduser("~/.cache/huggingface/hub")

model_slug = f"models--{MODEL_NAME.replace('/', '--')}"
snapshot_file = os.path.join(cache_dir, model_slug, "refs", "main")
if not os.path.exists(snapshot_file):
    # 尝试从 sentence_transformers 的内部缓存找
    import glob
    candidates = glob.glob(os.path.join(cache_dir, model_slug, "snapshots", "*"))
    if candidates:
        src_dir = sorted(candidates)[-1]
    else:
        print(f"错误: 找不到模型的缓存目录")
        print(f"请检查: {cache_dir}/{model_slug}/")
        sys.exit(1)
else:
    with open(snapshot_file) as f:
        snapshot = f.read().strip()
    src_dir = os.path.join(cache_dir, model_slug, "snapshots", snapshot)

print(f"复制模型文件: {src_dir} → {MODEL_DIR}")

# 复制所有模型文件
copied = 0
for fname in os.listdir(src_dir):
    src = os.path.join(src_dir, fname)
    dst = os.path.join(MODEL_DIR, fname)
    if os.path.isfile(src) and not os.path.exists(dst):
        shutil.copy2(src, dst)
        size_mb = os.path.getsize(dst) / (1024 * 1024)
        print(f"  {fname} ({size_mb:.1f} MB)")
        copied += 1

if copied == 0:
    print("所有文件已存在，无需复制")
else:
    print(f"\n完成！共 {copied} 个文件 → {MODEL_DIR}")
    total_mb = sum(os.path.getsize(os.path.join(MODEL_DIR, f)) for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
    print(f"总大小: {total_mb:.1f} MB")
    print(f"\n现在可以构建 Docker 镜像: docker compose build opsmind-server")
