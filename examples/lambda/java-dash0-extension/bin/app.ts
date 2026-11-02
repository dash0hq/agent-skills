#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { JavaDash0ExtensionStack } from '../lib/java-dash0-extension-stack';

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

new JavaDash0ExtensionStack(app, 'JavaDash0ExtensionStack', {
  env: { account, region },
  description:
    'Java Lambda instrumented by the Dash0 Lambda Extension layer (path B of the AWS Lambda decision process).',
  // OTLP/HTTP ingress endpoint, for example https://ingress.eu-west-1.aws.dash0.com:4318.
  dash0Endpoint: required('DASH0_ENDPOINT', 'https://ingress.eu-west-1.aws.dash0.com:4318'),
  // ARN of an existing Secrets Manager secret holding the Dash0 auth token as a plain string.
  dash0TokenSecretArn: required(
    'DASH0_TOKEN_SECRET_ARN',
    'arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf',
  ),
  dash0Dataset: process.env.DASH0_DATASET?.trim() || 'default',
});
