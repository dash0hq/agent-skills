import * as path from 'node:path';

import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import { Construct } from 'constructs';

/**
 * Version of the Dash0 Lambda Extension layer.
 *
 * Source: https://github.com/dash0hq/dash0-lambda-extension/releases
 * Latest release at lookup time: v27, looked up on 2026-10-29.
 *
 * The layer version suffix of the ARN is the release number itself, so release
 * `v27` is `...:layer:dash0-extension-node:27`.
 */
const DASH0_EXTENSION_LAYER_VERSION = 27;

/** AWS account the Dash0 Lambda Extension layers are published from. */
const DASH0_LAYER_ACCOUNT = '115813213817';

/**
 * The extension supports both `x86_64` and `arm64` and the ARN is not
 * architecture-scoped, unlike the upstream Collector layer.
 */
const ARCHITECTURE = lambda.Architecture.ARM_64;

const FUNCTION_NAME = 'TEST-nodejs-dash0-extension';

export interface Dash0ExtensionStackProps extends cdk.StackProps {
  /** OTLP/HTTP ingress endpoint, for example `https://ingress.eu-west-1.aws.dash0.com:4318`. */
  readonly dash0Endpoint: string;
  /** Dataset slug telemetry is routed to. */
  readonly dash0Dataset: string;
  /** ARN of the Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
}

export class Dash0ExtensionStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: Dash0ExtensionStackProps) {
    super(scope, id, props);

    if (cdk.Token.isUnresolved(this.region)) {
      throw new Error(
        'This stack must be synthesized with a concrete region: Lambda layer ARNs are region-scoped. ' +
          'Set CDK_DEFAULT_REGION or AWS_REGION.',
      );
    }

    // Path B of the decision process: Node.js runtime, Dash0 as the only
    // backend. One layer, no Collector layer, no configuration layer.
    const dash0Layer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'Dash0ExtensionLayer',
      `arn:aws:lambda:${this.region}:${DASH0_LAYER_ACCOUNT}:layer:dash0-extension-node:${DASH0_EXTENSION_LAYER_VERSION}`,
    );

    const logGroup = new logs.LogGroup(this, 'FunctionLogGroup', {
      logGroupName: `/aws/lambda/${FUNCTION_NAME}`,
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const fn = new lambda.Function(this, 'DemoFunction', {
      functionName: FUNCTION_NAME,
      runtime: lambda.Runtime.NODEJS_22_X,
      architecture: ARCHITECTURE,
      handler: 'index.handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'lambda')),
      memorySize: 512,
      timeout: cdk.Duration.seconds(30),
      logGroup,
      // `Active` makes the X-Ray SDK compete with OpenTelemetry for the trace
      // context and produces broken traces.
      tracing: lambda.Tracing.PASS_THROUGH,
      layers: [dash0Layer],
      environment: {
        // Without this the layer is attached but inert and only logs are collected.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/wrapper',
        DASH0_ENDPOINT: props.dash0Endpoint,
        DASH0_DATASET: props.dash0Dataset,
        // The token itself never enters the function configuration: the
        // extension reads it from Secrets Manager at runtime.
        DASH0_TOKEN_SECRET_ARN: props.dash0TokenSecretArn,
        OTEL_SERVICE_NAME: 'lambda-telemetry-demo',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=otel-examples,service.version=0.1.0,deployment.environment.name=test',
      },
    });

    // The extension resolves the token at runtime, so the execution role needs
    // read access to the secret. A trailing `*` tolerates both the complete ARN
    // (with the six-character Secrets Manager suffix) and the partial ARN.
    fn.addToRolePolicy(
      new iam.PolicyStatement({
        actions: ['secretsmanager:GetSecretValue'],
        resources: [props.dash0TokenSecretArn, `${props.dash0TokenSecretArn}*`],
      }),
    );

    const functionUrl = fn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'FunctionUrl', {
      value: functionUrl.url,
      description: 'Public Function URL; invoke it with curl to produce telemetry',
    });
    new cdk.CfnOutput(this, 'LogGroupName', {
      value: logGroup.logGroupName,
      description: 'CloudWatch log group carrying both the function and the extension output',
    });
    new cdk.CfnOutput(this, 'Dash0ExtensionLayerArn', {
      value: dash0Layer.layerVersionArn,
      description: 'Dash0 Lambda Extension layer attached to the function',
    });
  }
}
