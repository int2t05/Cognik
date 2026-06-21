"""
下载 cross-encoder 重排序模型文件。逐文件 HTTP 下载，无需 SDK。

用法：
    python models/rerank/download.py

模型：cross-encoder/ms-marco-MiniLM-L-4-v2（~80MB），轻量 cross-encoder。
优先 hf-mirror.com（国内镜像）→ huggingface.co。
"""

import os
import sys
import time

MODEL = "cross-encoder/ms-marco-MiniLM-L-4-v2"
MODEL_DIR = os.path.dirname(os.path.abspath(__file__))

# 镜像源（按优先级，国内优先）
MIRRORS = [
    "https://hf-mirror.com",
    "https://huggingface.co",
]

# 模型必需/可选文件
FILES = [
    "config.json",
    "model.safetensors",
    "tokenizer_config.json",
    "vocab.txt",
    "special_tokens_map.json",
]
OPTIONAL = [
    "sentence_bert_config.json",
    "modules.json",
    "tokenizer.json",
]

# 已下载则跳过
_REQUIRED = ["config.json", "model.safetensors", "vocab.txt"]
if all(os.path.exists(os.path.join(MODEL_DIR, f)) for f in _REQUIRED):
    total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
                for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
    print(f"模型已存在 ({total:.1f} MB)")
    sys.exit(0)

import requests

def try_download(url, dst, retries=3):
    """逐文件下载，带重试和完整性校验。"""
    for attempt in range(retries):
        try:
            resp = requests.get(url, stream=True, timeout=60)
            if resp.status_code == 200:
                total_size = int(resp.headers.get("content-length", 0))
                downloaded = 0
                print(f"  下载中... ({total_size/1024/1024:.1f} MB)", end="", flush=True)
                with open(dst, "wb") as f:
                    for chunk in resp.iter_content(chunk_size=65536):
                        f.write(chunk)
                        downloaded += len(chunk)
                actual = os.path.getsize(dst)
                if total_size > 0 and actual != total_size:
                    os.remove(dst)
                    print(f"\r  文件不完整 ({actual}/{total_size})，重试...")
                    continue
                print(f"\r  ok ({actual/1024/1024:.1f} MB)")
                return True
            elif resp.status_code == 404:
                return False
            else:
                print(f"  HTTP {resp.status_code}，重试...")
        except Exception as e:
            print(f"  错误: {e}，重试...")
        time.sleep(2 ** attempt)
    return False

# 逐文件下载
for file_list, required in [(FILES, True), (OPTIONAL, False)]:
    for fname in file_list:
        dst = os.path.join(MODEL_DIR, fname)
        if os.path.exists(dst):
            continue

        ok = False
        for mirror in MIRRORS:
            url = f"{mirror}/{MODEL}/resolve/main/{fname}"
            if try_download(url, dst):
                ok = True
                break

        if not ok:
            if required:
                print(f"\n错误: 无法下载必需文件 {fname}")
                sys.exit(1)

total = sum(os.path.getsize(os.path.join(MODEL_DIR, f))
            for f in os.listdir(MODEL_DIR) if os.path.isfile(os.path.join(MODEL_DIR, f))) / (1024 * 1024)
print(f"\n完成 ({total:.1f} MB)")
