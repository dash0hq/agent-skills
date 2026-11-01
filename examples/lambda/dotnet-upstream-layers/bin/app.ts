#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { DotnetUpstreamLayersStack } from '../lib/dotnet-upstream-layers-stack';

/**
 * Reads a required environment variable, or fails with an actionable message.
 */
function requireEnv(name: string, example: string): string {
  const value = process.env[name];
  if (value === undefined || value.trim() === '') {
    throw new Error(
      `Missing required environment variable ${name}.\n` +
        `  Example: export ${name}='${example}'\n` +
        '  See README.md, section "Configuration".',
    );
  }
  return value.trim();
}

// The Dash0 OTLP/gRPC ingress endpoint, host and port, without a scheme.
// Find it under Settings -> Endpoints in Dash0.
const dash0OtlpGrpcEndpoint = requireEnv('DASH0_OTLP_GRPC_ENDPOINT', 'ingress.eu-west-1.aws.dash0.com:4317');

// ARN of the AWS Secrets Manager secret holding the Dash0 auth token as a plain string.
const dash0TokenSecretArn = requireEnv(
  'DASH0_TOKEN_SECRET_ARN',
  'arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf',
);

// The Dash0 dataset telemetry is routed to.
const dash0Dataset = process.env.DASH0_DATASET?.trim() || 'default';

const app = new cdk.App();

new DotnetUpstreamLayersStack(app, 'Dash0DotnetUpstreamLayersStack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
  description:
    'A .NET Lambda function instrumented with the OpenTelemetry .NET SDK, exporting to the upstream OpenTelemetry Collector Lambda layer.',
  dash0OtlpGrpcEndpoint,
  dash0TokenSecretArn,
  dash0Dataset,
});

app.synth();
