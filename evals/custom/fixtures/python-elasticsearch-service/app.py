# Eval fixture: Python (Flask) HTTP service pre-instrumented through
# zero-code auto-instrumentation — the dependency pins in requirements.txt
# and the container command carry the instrumentation; this file stays plain
# application code (see evals/fixtures/README.md, "Pre-instrumented fixture
# exception", and the upgrade scenario built on this fixture for why the
# dependency set is the interesting part). It serves GET /checkout, performs
# one outbound HTTP call to DOWNSTREAM_URL while handling it, and best-effort
# indexes the order in Elasticsearch. The Elasticsearch host is synthetic and
# unreachable at eval time; the indexing call fails fast and is caught, so
# the service contract is unaffected. Listens on PORT (default 8080) and uses
# only obviously synthetic data (user@example.test, TEST-0001).
import logging
import os
import urllib.request

from elasticsearch import Elasticsearch
from flask import Flask, jsonify

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)

DOWNSTREAM_URL = os.environ.get("DOWNSTREAM_URL", "http://localhost:9090/inventory")
SEARCH_URL = os.environ.get("SEARCH_URL", "http://search.internal.example.test:9200")

es = Elasticsearch(SEARCH_URL, request_timeout=1, max_retries=0)


def index_order(order):
    # Best-effort search indexing: skipped when Elasticsearch is unreachable.
    try:
        es.index(index="orders", id=order["order_id"], document=order)
    except Exception:
        app.logger.info("order indexing skipped (Elasticsearch unreachable)")


@app.get("/checkout")
def checkout():
    with urllib.request.urlopen(DOWNSTREAM_URL, timeout=5) as response:
        inventory = response.read().decode("utf-8")

    order = dict(
        order_id="TEST-0001",
        customer_email="user@example.test",
        status="confirmed",
        inventory=inventory,
    )
    index_order(order)

    app.logger.info(
        "checkout completed order.id=%s customer.email=%s",
        "TEST-0001",
        "user@example.test",
    )

    return jsonify(**order)


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    app.run(host="0.0.0.0", port=port)
