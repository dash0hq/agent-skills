import * as path from 'node:path';

import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import { Construct } from 'constructs';

export interface CheckoutStackProps extends cdk.StackProps {
  /** Dash0 OTLP/HTTP ingress endpoint URL, for example `https://ingress.eu-west-1.aws.dash0.com`. */
  readonly dash0OtlpHttpEndpoint: string;
  /** Dash0 OTLP/gRPC ingress endpoint, host and port without a scheme, for example `ingress.eu-west-1.aws.dash0.com:4317`. */
  readonly dash0OtlpGrpcEndpoint: string;
  /** Dash0 dataset the telemetry must be routed to. */
  readonly dash0Dataset: string;
  /** ARN of the Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
  /**
   * Harness-owned resource attributes (carrying the per-run `test.id`) that
   * must reach every span the function emits.
   */
  readonly evalResourceAttributes: string;
}

// Harness-owned naming: the eval deployment role can only create resources
// whose names start with the stack name. Name every function or layer you add
// `${this.stackName}-<suffix>`.
export class CheckoutStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: CheckoutStackProps) {
    super(scope, id, props);

    const logGroup = new logs.LogGroup(this, 'CheckoutLogGroup', {
      logGroupName: `/aws/lambda/${this.stackName}`,
      retention: logs.RetentionDays.ONE_DAY,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const checkout = new lambda.Function(this, 'CheckoutFunction', {
      // Harness-owned: the function name is the stack name. Keep it.
      functionName: this.stackName,
      runtime: lambda.Runtime.PROVIDED_AL2023,
      architecture: lambda.Architecture.ARM_64,
      handler: 'bootstrap',
      // The directory holding the `bootstrap` binary `cargo lambda build` writes.
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'lambda', 'target', 'lambda', 'bootstrap')),
      memorySize: 512,
      timeout: cdk.Duration.seconds(30),
      logGroup,
      tracing: lambda.Tracing.ACTIVE,
      environment: {
        // Harness-owned: carries the per-run test.id the eval reads telemetry
        // back by. Keep this value; append further attributes after a comma.
        OTEL_RESOURCE_ATTRIBUTES: props.evalResourceAttributes,
      },
    });

    new cdk.CfnOutput(this, 'FunctionName', { value: checkout.functionName });
  }
}
