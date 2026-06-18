"""
从 ModelScope（魔搭社区）下载轻量 cross-encoder 模型到本地目录。

用法（在 server/ 目录下执行）：
    pip install modelscope
    python models/rerank/download.py

默认模型：ms-marco-MiniLM-L-4-v2（~50MB）
如需更换：RERANK_MODEL=ms-marco-MiniLM-L-6-v2 python models/rerank/download.py
"""

import os
import shutil
import sys

# ModelScope 上的模型 ID（ms-marco 系列由 sentence-transformers 官方维护）
MODEL_NAME = os.environ.get("RERANK_MODEL", "ms-marco-MiniLM-L-4-v2")
MODELSCOPE_PATH = f"AI-ModelScope/{MODEL_NAME}"

# 输出目录：与本脚本同级的模型文件目录
MODEL_DIR = os.path.dirname(os.path.abspath(__file__))

print(f"下载模型: {MODELSCOPE_PATH}")
print(f"目标目录: {MODEL_DIR}")

# 检查是否已下载
required_files = ["config.json", "model.safetensors", "tokenizer_config.json", "vocab.txt"]
if all(os.path.exists(os.path.join(MODEL_DIR, f)) for f in required_files):
    print("模型文件已存在，跳过下载")
    total_mb = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
                   for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
    print(f"总大小: {total_mb:.1f} MB")
    sys.exit(0)

try:
    from modelscope import snapshot_download
except ImportError:
    print("请先安装 modelscope: pip install modelscope")
    sys.exit(1)

print("正在从 ModelScope（魔搭社区）下载...")
print("（国内走阿里 CDN，速度稳定）")

# 下载到临时目录
cache_dir = snapshot_download(
    MODELSCOPE_PATH,
    cache_dir=os.path.join(MODEL_DIR, ".cache"),
)

# 复制模型文件到输出目录
copied = 0
for fname in os.listdir(cache_dir):
    src = os.path.join(cache_dir, fname)
    dst = os.path.join(MODEL_DIR, fname)
    if os.path.isfile(src) and not os.path.exists(dst):
        shutil.copy2(src, dst)
        size_mb = os.path.getsize(dst) / (1024 * 1024)
        print(f"  {fname} ({size_mb:.1f} MB)")
        copied += 1

# 清理缓存
shutil.rmtree(os.path.join(MODEL_DIR, ".cache"), ignore_errors=True)

if copied == 0:
    print("所有文件已存在")

total_mb = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
               for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
print(f"\n完成！{copied} 个文件 → {MODEL_DIR}")
print(f"总大小: {total_mb:.1f} MB")
print(f"\n现在可以构建 Docker 镜像: docker compose build opsmind-server")
