# Eval fixture: Python HTTP service pre-instrumented with an older
# OpenTelemetry release line, including the retired elasticsearch
# instrumentation (see evals/fixtures/README.md, "Pre-instrumented fixture
# exception", and the upgrade scenario built on this fixture).
# Deterministic build: pinned base image, pinned dependency versions.
#
#   docker build -t python-elasticsearch-service-eval .
FROM python:3.13-slim
ENV OTEL_SERVICE_NAME="python-search-service-eval"
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8080
CMD ["opentelemetry-instrument", "python", "app.py"]
