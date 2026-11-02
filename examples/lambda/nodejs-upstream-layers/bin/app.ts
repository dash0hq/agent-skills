#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';

import { UpstreamLayersStack } from '../lib/upstream-layers-stack';

function required(name: string, hint: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Environment variable ${name} is required. ${hint}`);
  }
  return value;
}

const dash0OtlpGrpcEndpoint = required(
  'DASH0_OTLP_GRPC_ENDPOINT',
  'Set it to the OTLP/gRPC ingress endpoint without a scheme, for example ingress.eu-west-1.aws.dash0.com:4317 ' +
    '(Dash0: Settings -> Endpoints).',
);

const dash0TokenSecretArn = required(
  'DASH0_TOKEN_SECRET_ARN',
  'Set it to the ARN of an existing AWS Secrets Manager secret whose value is the Dash0 auth token as a plain string.',
);

const dash0Dataset = process.env.DASH0_DATASET ?? 'default';

const account = process.env.CDK_DEFAULT_ACCOUNT ?? process.env.AWS_ACCOUNT_ID;
const region = process.env.CDK_DEFAULT_REGION ?? process.env.AWS_REGION;

if (!region) {
  throw new Error(
    'A region is required: Lambda layer ARNs are region-scoped and must name the function region. ' +
      'Set AWS_REGION or CDK_DEFAULT_REGION.',
  );
}

const app = new cdk.App();

new UpstreamLayersStack(app, 'TestNodejsUpstreamLayersStack', {
  env: { account, region },
  description:
    'Node.js Lambda instrumented with the upstream OpenTelemetry layers plus Collector layer (path A)',
  dash0OtlpGrpcEndpoint,
  dash0Dataset,
  dash0TokenSecretArn,
});

app.synth();
