import * as fs from 'fs';
import * as path from 'path';
import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import { Construct } from 'constructs';

/**
 * Upstream OpenTelemetry Lambda layer versions.
 *
 * Source: https://github.com/open-telemetry/opentelemetry-lambda/releases —
 * releases `layer-python/0.22.0` and `layer-collector/0.24.0`, both published
 * 2026-10-22. Looked up on 2026-10-29.
 *
 * Note the encoding: the version appears in the LAYER NAME with underscores
 * (`0_22_0`), and the trailing `:1` is the Lambda layer version, which is `1`
 * for every upstream publish. The `<version>` placeholder in the skill rule
 * file is written with dots, which does not resolve to a real layer.
 */
const OTEL_PYTHON_LAYER_NAME = 'opentelemetry-python-0_22_0';
const OTEL_COLLECTOR_LAYER_VERSION = '0_24_0';

/** Lambda layer version suffix used by every upstream publish. */
const UPSTREAM_LAYER_VERSION_SUFFIX = '1';

/** AWS account that publishes the upstream OpenTelemetry Lambda layers. */
const UPSTREAM_LAYER_ACCOUNT = '184161586896';

/**
 * Where the Collector layer looks for its configuration.
 *
 * AWS unpacks every layer into `/opt`, so the YAML must sit at the ROOT of the
 * configuration layer archive. A file inside a subdirectory lands at
 * `/opt/<subdir>/otel-collector-config.yaml`, and the Collector silently falls
 * back to its default configuration.
 */
const COLLECTOR_CONFIG_URI = '/opt/otel-collector-config.yaml';

export interface PythonUpstreamLayersStackProps extends cdk.StackProps {
  /** Dash0 OTLP/gRPC ingress endpoint, host and port only, for example `ingress.eu-west-1.aws.dash0.com:4317`. */
  readonly dash0GrpcEndpoint: string;
  /** Dash0 dataset the telemetry is routed to. */
  readonly dash0Dataset: string;
  /** ARN of an existing Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
}

export class PythonUpstreamLayersStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: PythonUpstreamLayersStackProps) {
    super(scope, id, props);

    const architecture = lambda.Architecture.ARM_64;

    // --- Configuration layer -------------------------------------------------
    //
    // The asset directory is generated here and contains exactly one file, so
    // CDK's directory zip puts `otel-collector-config.yaml` at the archive root.
    const configAssetDir = path.join(__dirname, '..', 'build', 'collector-config');
    fs.mkdirSync(configAssetDir, { recursive: true });
    fs.writeFileSync(
      path.join(configAssetDir, 'otel-collector-config.yaml'),
      renderCollectorConfig(props.dash0GrpcEndpoint, props.dash0Dataset),
      'utf8',
    );

    const configLayer = new lambda.LayerVersion(this, 'CollectorConfigLayer', {
      layerVersionName: `${this.stackName}-collector-config`,
      description: 'OpenTelemetry Collector configuration for the Lambda Collector layer.',
      code: lambda.Code.fromAsset(configAssetDir),
      compatibleArchitectures: [architecture],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    // --- Upstream layers -----------------------------------------------------
    //
    // Layers are region-scoped, and the Collector layer must match the
    // function's architecture.
    const instrumentationLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelPythonLayer',
      `arn:aws:lambda:${this.region}:${UPSTREAM_LAYER_ACCOUNT}:layer:${OTEL_PYTHON_LAYER_NAME}:${UPSTREAM_LAYER_VERSION_SUFFIX}`,
    );

    const collectorLayerArch = architecture === lambda.Architecture.ARM_64 ? 'arm64' : 'amd64';
    const collectorLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelCollectorLayer',
      `arn:aws:lambda:${this.region}:${UPSTREAM_LAYER_ACCOUNT}:layer:opentelemetry-collector-${collectorLayerArch}-${OTEL_COLLECTOR_LAYER_VERSION}:${UPSTREAM_LAYER_VERSION_SUFFIX}`,
    );

    // --- Function ------------------------------------------------------------
    //
    // The Collector configuration references the auth token as
    // `${env:DASH0_AUTH_TOKEN}`, so the token has to reach the function as an
    // environment variable. This is a CloudFormation dynamic reference: the
    // literal token never appears in the CDK source or the synthesized
    // template, but CloudFormation does resolve it into the function's
    // environment at deploy time. See the README for the tradeoff.
    const dash0AuthToken = cdk.SecretValue.secretsManager(props.dash0TokenSecretArn).unsafeUnwrap();

    const fn = new lambda.Function(this, 'TestFunction', {
      functionName: `${this.stackName}-test`,
      runtime: lambda.Runtime.PYTHON_3_13,
      architecture,
      handler: 'handler.lambda_handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'lambda')),
      memorySize: 512,
      timeout: cdk.Duration.seconds(30),
      tracing: lambda.Tracing.PASS_THROUGH,
      // Order of concern: instrumentation layer, Collector layer, configuration layer.
      layers: [instrumentationLayer, collectorLayer, configLayer],
      logGroup: new logs.LogGroup(this, 'TestFunctionLogGroup', {
        logGroupName: `/aws/lambda/${this.stackName}-test`,
        retention: logs.RetentionDays.ONE_WEEK,
        removalPolicy: cdk.RemovalPolicy.DESTROY,
      }),
      environment: {
        // Python uses `/opt/otel-instrument`. Node.js, Java, and Ruby use
        // `/opt/otel-handler`. Without this variable the layers are inert.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/otel-instrument',
        OPENTELEMETRY_COLLECTOR_CONFIG_URI: COLLECTOR_CONFIG_URI,
        OTEL_SERVICE_NAME: 'lambda-example',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=agent-skills-examples,service.version=0.1.0,deployment.environment.name=test',
        // Sample in the Collector, not the SDK.
        OTEL_TRACES_SAMPLER: 'always_on',
        // Read by the Collector configuration as ${env:DASH0_AUTH_TOKEN}.
        DASH0_AUTH_TOKEN: dash0AuthToken,
      },
    });

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
      description: 'CloudWatch log group holding both the handler and the Collector output.',
    });
  }
}

/**
 * Collector configuration for the `opentelemetry-lambda` Collector build.
 *
 * `telemetryapi` and `decouple` exist only in that build, not in
 * `otelcol-contrib`. `memory_limiter`, `sending_queue`, and `file_storage` are
 * deliberately absent: there is no durable filesystem inside the execution
 * environment, and shedding data in a process that is about to freeze loses it.
 */
function renderCollectorConfig(grpcEndpoint: string, dataset: string): string {
  return `# Generated by the CDK stack — do not edit by hand.
# Lives at the ROOT of the configuration layer archive, so it unpacks to
# /opt/otel-collector-config.yaml.
receivers:
  telemetryapi:
  otlp:
    protocols:
      grpc:
        endpoint: "localhost:4317"
      http:
        endpoint: "localhost:4318"

processors:
  decouple:

exporters:
  otlp_grpc/dash0:
    endpoint: ${grpcEndpoint}
    headers:
      Authorization: "Bearer \${env:DASH0_AUTH_TOKEN}"
      Dash0-Dataset: "${dataset}"

service:
  pipelines:
    traces:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [otlp_grpc/dash0]
    metrics:
      receivers: [otlp]
      processors: [decouple]
      exporters: [otlp_grpc/dash0]
    logs:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [otlp_grpc/dash0]
  telemetry:
    logs:
      level: info
`;
}
