#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { PythonDash0ExtensionStack } from '../lib/python-dash0-extension-stack';

function required(name: string, hint: string): string {
  const value = process.env[name];
  if (!value || value.trim() === '') {
    throw new Error(`Missing required environment variable ${name}. ${hint}`);
  }
  return value.trim();
}

const dash0Endpoint = required(
  'DASH0_ENDPOINT',
  'Set it to the Dash0 OTLP/HTTP ingress endpoint, for example https://ingress.eu-west-1.aws.dash0.com:4318 (Settings -> Endpoints in Dash0).',
);

if (!dash0Endpoint.startsWith('https://')) {
  throw new Error(`DASH0_ENDPOINT must be an https:// URL, got "${dash0Endpoint}".`);
}

const dash0TokenSecretArn = required(
  'DASH0_TOKEN_SECRET_ARN',
  'Set it to the ARN of an existing AWS Secrets Manager secret whose value is the Dash0 auth token as a plain string.',
);

if (!dash0TokenSecretArn.startsWith('arn:aws:secretsmanager:')) {
  throw new Error(
    `DASH0_TOKEN_SECRET_ARN must be a full Secrets Manager secret ARN including the six-character suffix, got "${dash0TokenSecretArn}".`,
  );
}

const dash0Dataset = process.env.DASH0_DATASET?.trim() || 'default';

const region = process.env.CDK_DEFAULT_REGION ?? process.env.AWS_REGION;
const account = process.env.CDK_DEFAULT_ACCOUNT;

if (!region) {
  throw new Error(
    'No AWS region resolved. Export AWS_REGION (or configure a region in your AWS profile) so the region-scoped layer ARN can be built.',
  );
}

const app = new cdk.App();

new PythonDash0ExtensionStack(app, 'PythonDash0ExtensionStack', {
  env: { account, region },
  description:
    'Python Lambda instrumented with the Dash0 Lambda Extension layer (path B of the AWS Lambda decision process).',
  dash0Endpoint,
  dash0Dataset,
  dash0TokenSecretArn,
});
