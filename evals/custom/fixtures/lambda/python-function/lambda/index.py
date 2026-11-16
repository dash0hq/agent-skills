"""Checkout handler: confirms the payment account with one outbound AWS API call
and returns a synthetic order. There is deliberately no OpenTelemetry code here.

boto3 ships with the AWS Lambda Python runtime, so the deployment package has no
dependencies. STS GetCallerIdentity needs no IAM permission.
"""

import json

import boto3

_sts = boto3.client("sts")


def handler(event, context):
    _sts.get_caller_identity()
    print(json.dumps({"message": "checkout completed", "order.id": "TEST-0001"}))
    return {
        "statusCode": 200,
        "body": json.dumps(
            {
                "order_id": "TEST-0001",
                "customer_email": "user@example.test",
                "status": "confirmed",
            }
        ),
    }
