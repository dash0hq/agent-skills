"""Test handler for the AWS Lambda OpenTelemetry examples.

There is deliberately NO OpenTelemetry SDK code in this file.
Both example projects rely entirely on layer-based auto-instrumentation, so any
bootstrap code here would hide whether the layers actually work.

Only the Python standard library and the `boto3`/`botocore` packages that the
AWS Lambda Python runtime already provides are used, so the deployment package
needs no `pip install`.
"""

import hashlib
import json
import os
import time
import urllib.error
import urllib.request

# `boto3` ships with the AWS Lambda Python runtime, so importing it costs no
# extra dependency. It is instrumented by both the upstream OpenTelemetry Python
# layer (via `opentelemetry-instrumentation-botocore`) and the Dash0 extension,
# which is not true of `urllib`: see the "Known gaps" section of the README.
import boto3

UNRESOLVABLE_URL = "https://example.test/"
REACHABLE_URL = "https://aws.amazon.com/"
HTTP_TIMEOUT_SECONDS = 5


def _burn_cpu(rounds: int = 200_000) -> str:
    """Spend a measurable amount of CPU so the invocation has a visible duration."""
    digest = b"TEST-0001"
    for _ in range(rounds):
        digest = hashlib.sha256(digest).digest()
    return digest.hex()[:16]


def _fetch(url: str) -> dict:
    """Fetch a URL and report the outcome instead of raising."""
    started = time.monotonic()
    try:
        with urllib.request.urlopen(url, timeout=HTTP_TIMEOUT_SECONDS) as response:
            body = response.read(256)
            return {
                "url": url,
                "ok": True,
                "status": response.status,
                "bytes_read": len(body),
                "duration_ms": round((time.monotonic() - started) * 1000, 2),
            }
    except Exception as error:  # noqa: BLE001 - the failure is the point
        return {
            "url": url,
            "ok": False,
            "error_type": type(error).__name__,
            "error": str(error),
            "duration_ms": round((time.monotonic() - started) * 1000, 2),
        }


def _call_aws() -> dict:
    """Make one AWS SDK call so there is always an instrumented child span."""
    try:
        identity = boto3.client("sts").get_caller_identity()
        return {"ok": True, "arn_suffix": identity["Arn"].rsplit("/", 1)[-1]}
    except Exception as error:  # noqa: BLE001
        return {"ok": False, "error_type": type(error).__name__}


def lambda_handler(event, context):
    print("TEST-0001 invocation start")

    checksum = _burn_cpu()

    # This one fails DNS resolution on purpose: `.test` is a reserved TLD, so the
    # resulting span carries an ERROR status.
    failing = _fetch(UNRESOLVABLE_URL)
    print(f"TEST-0001 expected failure: {failing['error_type']}")

    # This one succeeds, so there is a child span with an OK/UNSET status next to it.
    succeeding = _fetch(REACHABLE_URL)
    print(f"TEST-0001 reachable call ok={succeeding['ok']}")

    aws_call = _call_aws()

    body = {
        "message": "TEST-0001 ok",
        "checksum": checksum,
        "function": os.environ.get("AWS_LAMBDA_FUNCTION_NAME", "unknown"),
        "service": os.environ.get("OTEL_SERVICE_NAME", "unset"),
        "request_id": getattr(context, "aws_request_id", "unknown"),
        "outbound": [failing, succeeding],
        "aws_sdk_call": aws_call,
    }

    print("TEST-0001 invocation end")

    return {
        "statusCode": 200,
        "headers": {"content-type": "application/json"},
        "body": json.dumps(body),
    }
