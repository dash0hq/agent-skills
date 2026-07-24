# Eval fixture: deliberately uninstrumented Python (Flask) HTTP service (see
# evals/fixtures/README.md for the contract). It serves GET /checkout,
# performs one outbound HTTP call to DOWNSTREAM_URL while handling it, listens
# on PORT (default 8080), and uses only obviously synthetic data
# (user@example.test, TEST-0001). Flask is the only dependency; the outbound
# call uses the standard library.
import logging
import os
import urllib.request

from flask import Flask, jsonify

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)

DOWNSTREAM_URL = os.environ.get("DOWNSTREAM_URL", "http://localhost:9090/inventory")


@app.get("/checkout")
def checkout():
    with urllib.request.urlopen(DOWNSTREAM_URL, timeout=5) as response:
        inventory = response.read().decode("utf-8")

    app.logger.info(
        "checkout completed order.id=%s customer.email=%s",
        "TEST-0001",
        "user@example.test",
    )

    return jsonify(
        order_id="TEST-0001",
        customer_email="user@example.test",
        status="confirmed",
        inventory=inventory,
    )


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    app.run(host="0.0.0.0", port=port)
