#
# Build context: repo root.
#   docker build -f deploy/embeddings.Dockerfile -t projectx-embeddings .
#
# CPU-only build: this box has no GPU, but the default PyPI `torch` wheel is
# the CUDA-enabled build — installing it drags in several hundred MB of
# unused NVIDIA libraries and bloats/slows the image for no benefit. Install
# torch from PyTorch's CPU-only index first; the rest of requirements.txt
# then installs normally against PyPI (pip sees torch already satisfies the
# "torch==2.*" pin and leaves it alone).
#
FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1

COPY apps/embeddings/requirements.txt ./
RUN pip install --no-cache-dir torch==2.* --index-url https://download.pytorch.org/whl/cpu \
 && pip install --no-cache-dir -r requirements.txt

COPY apps/embeddings/main.py ./

# Hugging Face's model cache — mounted as a named volume in compose so the
# ~1.1GB model isn't re-downloaded every time this container is recreated
# (e.g. on every `deploy.sh` run).
ENV HF_HOME=/cache/huggingface
RUN mkdir -p /cache/huggingface

EXPOSE 8001
CMD ["python", "main.py"]
