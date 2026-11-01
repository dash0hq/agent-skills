#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { JavaUpstreamLayersStack } from '../lib/java-upstream-layers-stack';

function required(name: string, example: string): string {
  const value = process.env[name];
  if (!value || value.trim() === '') {
    throw new Error(
      `Missing required environment variable ${name}. Export it before synthesising or deploying, for example:\n` +
        `  export ${name}='${example}'`,
    );
  }
  return value.trim();
}

const region = process.env.CDK_DEFAULT_REGION ?? process.env.AWS_REGION;
const account = process.env.CDK_DEFAULT_ACCOUNT;

if (!region) {
  throw new Error(
    'No AWS region resolved. Set AWS_REGION, or configure a default region in your AWS profile.',
  );
}

const app = new cdk.App();

new JavaUpstreamLayersStack(app, 'JavaUpstreamLayersStack', {
  env: { account, region },
  description:
    'Java Lambda instrumented by the upstream OpenTelemetry Lambda layers (path A of the AWS Lambda decision process).',
  // Host and port only -- the OTLP/gRPC exporter takes no scheme.
  dash0OtlpGrpcEndpoint: required('DASH0_OTLP_GRPC_ENDPOINT', 'ingress.eu-west-1.aws.dash0.com:4317'),
  // ARN of an existing Secrets Manager secret holding the Dash0 auth token as a plain string.
  dash0TokenSecretArn: required(
    'DASH0_TOKEN_SECRET_ARN',
    'arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf',
  ),
  dash0Dataset: process.env.DASH0_DATASET?.trim() || 'default',
});
