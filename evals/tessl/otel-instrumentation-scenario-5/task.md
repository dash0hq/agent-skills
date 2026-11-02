# Instrumenting a Node.js AWS Lambda Function for Dash0

## Problem/Feature Description

A logistics company runs its parcel-tracking API as a set of AWS Lambda functions deployed with AWS SAM. The `TrackingLookupFunction` is a Node.js 22 function on the `arm64` architecture, deployed to `eu-west-1`, packaged as a ZIP archive. It currently emits nothing but the default CloudWatch platform logs, and the on-call team has no visibility into why some lookups take seconds.

The company already uses Dash0 as its observability backend and wants this function instrumented with no changes to the handler code. Telemetry must reach the `production` dataset. The Dash0 auth token is already stored in AWS Secrets Manager as a plain-string secret; the SAM template exposes its ARN through the existing `Dash0TokenSecret` resource, and the security team has made it clear that the token must never appear as a literal value in the template, in the repository, or in the function's environment configuration.

Several functions make up the parcel-tracking service, so telemetry from this function must be attributed to the service `parcel-tracking`, not to the function name. The function's X-Ray tracing mode is currently set to `Active` from an earlier experiment.

The current function definition in `template.yaml` is:

```yaml
Resources:
  TrackingLookupFunction:
    Type: AWS::Serverless::Function
    Properties:
      Runtime: nodejs22.x
      Architectures: [arm64]
      Handler: index.handler
      CodeUri: src/
      Tracing: Active
```

## Output Specification

Produce an updated `template.yaml` in which `TrackingLookupFunction` is instrumented end to end, covering:

- The Lambda layer or layers needed to collect traces, metrics, and logs
- The environment variables that activate instrumentation and route telemetry to the correct Dash0 dataset
- How the function obtains the auth token, including any IAM permission it needs
- The service identity carried on the telemetry
- Any change needed to existing properties that would conflict with OpenTelemetry

Also produce a short `INSTRUMENTATION.md` explaining how to verify that telemetry is arriving after deploying.
