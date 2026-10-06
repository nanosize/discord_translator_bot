FROM python:3.10-slim

# システム依存関係（必要に応じて追加）
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 依存関係インストール
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# アプリケーションコード
COPY . .

# コンテナ起動時に bot を起動
CMD ["python", "bot.py"]
