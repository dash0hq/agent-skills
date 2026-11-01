#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { PythonUpstreamLayersStack } from '../lib/python-upstream-layers-stack';

function required(name: string, hint: string): string {
  const value = process.env[name];
  if (!value || value.trim() === '') {
    throw new Error(`Missing required environment variable ${name}. ${hint}`);
  }
  return value.trim();
}

const dash0GrpcEndpoint = required(
  'DASH0_OTLP_GRPC_ENDPOINT',
  'Set it to the Dash0 OTLP/gRPC ingress endpoint as host:port, for example ingress.eu-west-1.aws.dash0.com:4317 (Settings -> Endpoints in Dash0).',
);

if (dash0GrpcEndpoint.includes('://')) {
  throw new Error(
    `DASH0_OTLP_GRPC_ENDPOINT must be host:port without a scheme, got "${dash0GrpcEndpoint}". The Collector otlp exporter takes a bare endpoint and uses TLS by default.`,
  );
}

if (!/:\d+$/.test(dash0GrpcEndpoint)) {
  throw new Error(
    `DASH0_OTLP_GRPC_ENDPOINT must end in a port, for example ingress.eu-west-1.aws.dash0.com:4317, got "${dash0GrpcEndpoint}".`,
  );
}

const dash0TokenSecretArn = required(
  'DASH0_TOKEN_SECRET_ARN',
  'Set it to the ARN of an existing AWS Secrets Manager secret whose value is the Dash0 auth token as a plain string.',
);

if (!dash0TokenSecretArn.startsWith('arn:aws:secretsmanager:')) {
  throw new Error(
    `DASH0_TOKEN_SECRET_ARN must be a full Secrets Manager secret ARN, got "${dash0TokenSecretArn}".`,
  );
}

const dash0Dataset = process.env.DASH0_DATASET?.trim() || 'default';

const region = process.env.CDK_DEFAULT_REGION ?? process.env.AWS_REGION;
const account = process.env.CDK_DEFAULT_ACCOUNT;

if (!region) {
  throw new Error(
    'No AWS region resolved. Export AWS_REGION (or configure a region in your AWS profile) so the region-scoped layer ARNs can be built.',
  );
}

const app = new cdk.App();

new PythonUpstreamLayersStack(app, 'PythonUpstreamLayersStack', {
  env: { account, region },
  description:
    'Python Lambda instrumented with the upstream OpenTelemetry layers (path A of the AWS Lambda decision process).',
  dash0GrpcEndpoint,
  dash0Dataset,
  dash0TokenSecretArn,
});
