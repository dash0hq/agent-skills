import * as path from 'node:path';

import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import { Construct } from 'constructs';

/**
 * Versions of the upstream opentelemetry-lambda layers.
 *
 * Source: https://github.com/open-telemetry/opentelemetry-lambda/releases
 * Looked up on 2026-10-29: latest releases are `layer-nodejs/0.24.0` and
 * `layer-collector/0.24.0`, both published 2026-10-22.
 *
 * The version appears in the layer NAME with underscores, not dots, and the
 * layer version suffix of the ARN is always `:1`. The release notes of each
 * release carry the exact ARN template:
 *
 *   arn:aws:lambda:<region>:184161586896:layer:opentelemetry-nodejs-0_24_0:1
 *   arn:aws:lambda:<region>:184161586896:layer:opentelemetry-collector-<amd64|arm64>-0_24_0:1
 */
const OTEL_NODEJS_LAYER_VERSION = '0_24_0';
const OTEL_COLLECTOR_LAYER_VERSION = '0_24_0';
const OTEL_LAYER_ARN_SUFFIX = '1';

/** AWS account the upstream OpenTelemetry Lambda layers are published from. */
const OTEL_LAYER_ACCOUNT = '184161586896';

/**
 * The Collector layer ARN is architecture-scoped. Changing this constant
 * requires changing `COLLECTOR_LAYER_ARCH_SLUG` with it, or the extension
 * binary never starts and the function exports nothing while still succeeding.
 */
const ARCHITECTURE = lambda.Architecture.ARM_64;
const COLLECTOR_LAYER_ARCH_SLUG = 'arm64';

const FUNCTION_NAME = 'TEST-nodejs-upstream-layers';

export interface UpstreamLayersStackProps extends cdk.StackProps {
  /** Dash0 OTLP/gRPC ingress endpoint, for example `ingress.eu-west-1.aws.dash0.com:4317`. */
  readonly dash0OtlpGrpcEndpoint: string;
  /** Dataset slug telemetry is routed to. */
  readonly dash0Dataset: string;
  /** ARN of the Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
}

export class UpstreamLayersStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: UpstreamLayersStackProps) {
    super(scope, id, props);

    if (cdk.Token.isUnresolved(this.region)) {
      throw new Error(
        'This stack must be synthesized with a concrete region: Lambda layer ARNs are region-scoped. ' +
          'Set CDK_DEFAULT_REGION or AWS_REGION.',
      );
    }

    // Path A of the decision process: three layers, in this order of concern.
    // 1. The language instrumentation layer.
    const instrumentationLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelNodejsLayer',
      `arn:aws:lambda:${this.region}:${OTEL_LAYER_ACCOUNT}:layer:opentelemetry-nodejs-${OTEL_NODEJS_LAYER_VERSION}:${OTEL_LAYER_ARN_SUFFIX}`,
    );

    // 2. The Collector layer, matching the function architecture.
    const collectorLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelCollectorLayer',
      `arn:aws:lambda:${this.region}:${OTEL_LAYER_ACCOUNT}:layer:opentelemetry-collector-${COLLECTOR_LAYER_ARCH_SLUG}-${OTEL_COLLECTOR_LAYER_VERSION}:${OTEL_LAYER_ARN_SUFFIX}`,
    );

    // 3. The configuration layer holding the Collector configuration.
    //
    // `Code.fromAsset` on a directory zips the directory CONTENTS, so
    // `collector-config-layer/otel-collector-config.yaml` lands at the archive
    // root and unpacks to `/opt/otel-collector-config.yaml`. Do not nest the
    // YAML in a subdirectory: the Collector would silently fall back to its
    // default configuration.
    const configLayer = new lambda.LayerVersion(this, 'OtelCollectorConfigLayer', {
      layerVersionName: 'TEST-otel-collector-config',
      description: 'Collector configuration consumed through OPENTELEMETRY_COLLECTOR_CONFIG_URI',
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'collector-config-layer')),
      compatibleArchitectures: [ARCHITECTURE],
      compatibleRuntimes: [lambda.Runtime.NODEJS_22_X],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

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
      memorySize: 1024,
      timeout: cdk.Duration.seconds(30),
      logGroup,
      tracing: lambda.Tracing.PASS_THROUGH,
      layers: [instrumentationLayer, collectorLayer, configLayer],
      environment: {
        // Without this the layers are attached but inert.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/otel-handler',
        OPENTELEMETRY_COLLECTOR_CONFIG_URI: '/opt/otel-collector-config.yaml',
        OTEL_SERVICE_NAME: 'lambda-telemetry-demo',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=otel-examples,service.version=0.1.0,deployment.environment.name=test',
        // Sample in the Collector, not the SDK.
        OTEL_TRACES_SAMPLER: 'always_on',
        // The handler uses global fetch (undici). The upstream layer bundles the
        // undici instrumentation but does not enable it by default, and does enable
        // `net` -- so without this the fetch calls produce only tcp.connect and
        // tls.connect spans and no HTTP client span. This variable is an allowlist
        // that replaces the default set, so `http` and `aws-sdk` are named too.
        OTEL_NODE_ENABLED_INSTRUMENTATIONS: 'http,undici,aws-sdk',
        // Consumed by `${env:...}` placeholders in otel-collector-config.yaml.
        DASH0_OTLP_GRPC_ENDPOINT: props.dash0OtlpGrpcEndpoint,
        DASH0_DATASET: props.dash0Dataset,
        // See the README: the Collector layer can only read the token from the
        // process environment, so CloudFormation resolves the secret at deploy
        // time and the plaintext token lands in the function configuration.
        DASH0_AUTH_TOKEN: cdk.SecretValue.secretsManager(props.dash0TokenSecretArn).unsafeUnwrap(),
      },
    });

    const functionUrl = fn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'FunctionUrl', {
      value: functionUrl.url,
      description: 'Public Function URL; invoke it with curl to produce telemetry',
    });
    new cdk.CfnOutput(this, 'LogGroupName', {
      value: logGroup.logGroupName,
      description: 'CloudWatch log group carrying both the function and the Collector output',
    });
    new cdk.CfnOutput(this, 'InstrumentationLayerArn', {
      value: instrumentationLayer.layerVersionArn,
    });
    new cdk.CfnOutput(this, 'CollectorLayerArn', {
      value: collectorLayer.layerVersionArn,
    });
    new cdk.CfnOutput(this, 'CollectorConfigLayerArn', {
      value: configLayer.layerVersionArn,
    });
  }
}
