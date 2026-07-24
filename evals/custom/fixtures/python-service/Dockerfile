# Eval fixture: uninstrumented Python HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base image, pinned dependency versions.
#
#   docker build -t python-service-eval .
FROM python:3.13-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8080
CMD ["python", "app.py"]
