#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';

import { CheckoutStack } from '../lib/checkout-stack';

// Harness-owned inputs: the eval harness sets every EVAL_* and DASH0_*
// variable below at deploy time. Keep this file's handling of them unchanged.
function required(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Environment variable ${name} is required; the eval harness sets it at deploy time.`);
  }
  return value;
}

const app = new cdk.App();

const stack = new CheckoutStack(app, 'CheckoutStack', {
  stackName: required('EVAL_STACK_NAME'),
  env: { account: process.env.CDK_DEFAULT_ACCOUNT, region: required('CDK_DEFAULT_REGION') },
  dash0Endpoint: required('DASH0_ENDPOINT'),
  dash0Dataset: required('DASH0_DATASET'),
  dash0TokenSecretArn: required('DASH0_TOKEN_SECRET_ARN'),
  evalResourceAttributes: required('EVAL_RESOURCE_ATTRIBUTES'),
});

// Every IAM role the stack creates must carry the eval permissions boundary;
// the deployment is refused otherwise.
iam.PermissionsBoundary.of(stack).apply(
  iam.ManagedPolicy.fromManagedPolicyName(stack, 'EvalPermissionsBoundary', required('EVAL_PERMISSIONS_BOUNDARY')),
);

app.synth();
