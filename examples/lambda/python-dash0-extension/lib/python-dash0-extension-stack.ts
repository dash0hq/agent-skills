import * as path from 'path';
import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * Version of the Dash0 Lambda Extension layer.
 *
 * Source: https://github.com/dash0hq/dash0-lambda-extension/releases — release
 * `v27`, published 2026-10-27. Looked up on 2026-10-29.
 *
 * The AWS Lambda deployment rule says to resolve this from the releases page
 * rather than copying it out of documentation, because layer versions move
 * independently of the docs that mention them.
 */
const DASH0_EXTENSION_LAYER_VERSION = 27;

/** AWS account that publishes the Dash0 Lambda Extension layers. */
const DASH0_LAYER_ACCOUNT = '115813213817';

export interface PythonDash0ExtensionStackProps extends cdk.StackProps {
  /** Dash0 OTLP/HTTP ingress endpoint, for example `https://ingress.eu-west-1.aws.dash0.com:4318`. */
  readonly dash0Endpoint: string;
  /** Dash0 dataset the telemetry is routed to. */
  readonly dash0Dataset: string;
  /** ARN of an existing Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
}

export class PythonDash0ExtensionStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: PythonDash0ExtensionStackProps) {
    super(scope, id, props);

    const tokenSecret = secretsmanager.Secret.fromSecretCompleteArn(
      this,
      'Dash0TokenSecret',
      props.dash0TokenSecretArn,
    );

    // Path B attaches exactly one layer. The upstream Collector layer and any
    // other vendor layer must not be attached alongside it.
    const dash0Layer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'Dash0ExtensionLayer',
      `arn:aws:lambda:${this.region}:${DASH0_LAYER_ACCOUNT}:layer:dash0-extension-python:${DASH0_EXTENSION_LAYER_VERSION}`,
    );

    const fn = new lambda.Function(this, 'TestFunction', {
      functionName: `${this.stackName}-test`,
      runtime: lambda.Runtime.PYTHON_3_13,
      architecture: lambda.Architecture.ARM_64,
      handler: 'handler.lambda_handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'lambda')),
      memorySize: 512,
      timeout: cdk.Duration.seconds(30),
      // `Active` makes the X-Ray SDK compete with OpenTelemetry for the trace
      // context and produces broken traces.
      tracing: lambda.Tracing.PASS_THROUGH,
      layers: [dash0Layer],
      logGroup: new logs.LogGroup(this, 'TestFunctionLogGroup', {
        logGroupName: `/aws/lambda/${this.stackName}-test`,
        retention: logs.RetentionDays.ONE_WEEK,
        removalPolicy: cdk.RemovalPolicy.DESTROY,
      }),
      environment: {
        // Without this the layer is attached but inert: the extension would
        // collect logs only, and no traces.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/wrapper',
        DASH0_ENDPOINT: props.dash0Endpoint,
        DASH0_DATASET: props.dash0Dataset,
        DASH0_TOKEN_SECRET_ARN: props.dash0TokenSecretArn,
        // Several functions can make up one logical service; without this,
        // `service.name` falls back to the function name.
        OTEL_SERVICE_NAME: 'lambda-example',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=agent-skills-examples,service.version=0.1.0,deployment.environment.name=test',
      },
    });

    // The extension reads the token from Secrets Manager at runtime, using the
    // function's execution role.
    tokenSecret.grantRead(fn);

    const functionUrl = fn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'FunctionUrl', {
      value: functionUrl.url,
      description: 'Public Function URL — curl this to produce telemetry.',
    });

    new cdk.CfnOutput(this, 'FunctionName', {
      value: fn.functionName,
      description: 'Lambda function name.',
    });

    new cdk.CfnOutput(this, 'LogGroupName', {
      value: `/aws/lambda/${fn.functionName}`,
      description: 'CloudWatch log group holding both the handler and the extension output.',
    });
  }
}
