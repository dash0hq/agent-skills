import * as fs from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * Version of the upstream OpenTelemetry Collector Lambda layer.
 *
 * Source: https://github.com/open-telemetry/opentelemetry-lambda/releases, release
 * `layer-collector/0.24.0`, published 2026-10-22. Looked up on 2026-10-29.
 *
 * Note the ARN encoding: the layer name carries the version with underscores
 * (`0_24_0`), and the trailing `:1` is the layer version number, not the release.
 * Re-resolve this from the releases page rather than trusting the value here.
 */
const COLLECTOR_LAYER_VERSION = '0.24.0';

/** AWS account that publishes the upstream OpenTelemetry Lambda layers. */
const UPSTREAM_LAYER_ACCOUNT = '184161586896';

/** Lambda layers are region- and architecture-scoped; both must match the function. */
const ARCHITECTURE = lambda.Architecture.ARM_64;
const COLLECTOR_LAYER_ARCH = 'arm64';

/**
 * Directory holding the `bootstrap` executable, produced by `npm run build:function`
 * (`cargo lambda build --release --arm64`). The provided.al2023 runtime starts the
 * executable with that name.
 */
const FUNCTION_BUILD_DIR = path.join(__dirname, '..', 'lambda', 'target', 'lambda', 'bootstrap');

/**
 * Directory whose contents become the configuration layer. CDK zips the *contents* of
 * the directory, so `otel-collector-config.yaml` sits directly in it and therefore lands
 * at the root of the archive — and at `/opt/otel-collector-config.yaml` once AWS unpacks
 * the layer. A nested subdirectory would make the Collector silently fall back to its
 * built-in configuration.
 */
const COLLECTOR_CONFIG_DIR = path.join(__dirname, '..', 'collector-config');

export interface RustUpstreamLayersStackProps extends cdk.StackProps {
  /** Dash0 OTLP/gRPC ingress endpoint, host and port, no scheme. */
  readonly dash0OtlpGrpcEndpoint: string;
  /** ARN of the Secrets Manager secret holding the Dash0 auth token as a plain string. */
  readonly dash0TokenSecretArn: string;
  /** Dash0 dataset telemetry is routed to. */
  readonly dash0Dataset: string;
}

export class RustUpstreamLayersStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: RustUpstreamLayersStackProps) {
    super(scope, id, props);

    if (!fs.existsSync(path.join(FUNCTION_BUILD_DIR, 'bootstrap'))) {
      throw new Error(
        `The Rust function has not been built to ${FUNCTION_BUILD_DIR}.\n` +
          '  Run `npm run build:function` before synthesizing or deploying.\n' +
          '  See README.md, section "Build the function".',
      );
    }

    // ---------------------------------------------------------------------------
    // Layers
    // ---------------------------------------------------------------------------

    // There is no upstream auto-instrumentation layer for Rust, so only two layers are
    // attached: the Collector and its configuration. The instrumentation lives in
    // lambda/src/main.rs.
    const collectorLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelCollectorLayer',
      `arn:aws:lambda:${this.region}:${UPSTREAM_LAYER_ACCOUNT}:layer:opentelemetry-collector-${COLLECTOR_LAYER_ARCH}-${COLLECTOR_LAYER_VERSION.replace(/\./g, '_')}:1`,
    );

    const collectorConfigLayer = new lambda.LayerVersion(this, 'OtelCollectorConfigLayer', {
      layerVersionName: `${cdk.Stack.of(this).stackName}-otel-collector-config`,
      description: 'OpenTelemetry Collector configuration for the Lambda Collector layer.',
      code: lambda.Code.fromAsset(COLLECTOR_CONFIG_DIR),
      compatibleArchitectures: [ARCHITECTURE],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    // ---------------------------------------------------------------------------
    // Auth token
    // ---------------------------------------------------------------------------

    // The Collector configuration reads the token as ${env:DASH0_AUTH_TOKEN}, so the
    // token has to be an environment variable on the function. CloudFormation resolves
    // this dynamic reference at deploy time, so the token never appears in the template
    // or in this repository — but the resolved value is then stored in the function
    // configuration. See README.md, section "How the auth token reaches the Collector".
    const dash0TokenSecret = secretsmanager.Secret.fromSecretCompleteArn(
      this,
      'Dash0TokenSecret',
      props.dash0TokenSecretArn,
    );

    // ---------------------------------------------------------------------------
    // Function
    // ---------------------------------------------------------------------------

    const logGroup = new logs.LogGroup(this, 'Dash0RustDemoLogGroup', {
      logGroupName: `/aws/lambda/${this.stackName}-rust-demo`,
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const fn = new lambda.Function(this, 'Dash0RustDemoFunction', {
      runtime: lambda.Runtime.PROVIDED_AL2023,
      architecture: ARCHITECTURE,
      // Ignored by the OS-only runtime, which always starts /var/task/bootstrap.
      handler: 'bootstrap',
      code: lambda.Code.fromAsset(FUNCTION_BUILD_DIR),
      memorySize: 256,
      timeout: cdk.Duration.seconds(30),
      // The Collector extension needs a cold start to register; PassThrough keeps the
      // X-Ray SDK from competing with OpenTelemetry for the trace context.
      tracing: lambda.Tracing.PASS_THROUGH,
      layers: [collectorLayer, collectorConfigLayer],
      // Both the handler's own output and the Collector extension's output land here.
      logGroup,
      environment: {
        // --- Collector layer ---
        // Points the Collector extension at the YAML in the configuration layer.
        OPENTELEMETRY_COLLECTOR_CONFIG_URI: '/opt/otel-collector-config.yaml',
        // Consumed by ${env:...} placeholders inside that YAML.
        DASH0_OTLP_GRPC_ENDPOINT: props.dash0OtlpGrpcEndpoint,
        DASH0_DATASET: props.dash0Dataset,
        DASH0_AUTH_TOKEN: dash0TokenSecret.secretValue.unsafeUnwrap(),

        // --- OpenTelemetry Rust SDK ---
        // Export to the Collector layer over loopback, never straight to the backend:
        // a remote round trip would run inside the billed invocation.
        OTEL_EXPORTER_OTLP_ENDPOINT: 'http://localhost:4318',
        OTEL_EXPORTER_OTLP_PROTOCOL: 'http/protobuf',
        OTEL_SERVICE_NAME: 'dash0-rust-demo',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=examples,service.version=0.1.0,deployment.environment.name=development',
        // Sample in the Collector, not in the SDK.
        OTEL_TRACES_SAMPLER: 'always_on',
      },
    });

    // Not strictly required for the dynamic reference above (CloudFormation resolves it
    // with the deploying principal's permissions), but granted so the function can also
    // read the secret directly if the example is extended to fetch it at runtime.
    dash0TokenSecret.grantRead(fn);

    const fnUrl = fn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    // ---------------------------------------------------------------------------
    // Outputs
    // ---------------------------------------------------------------------------

    new cdk.CfnOutput(this, 'FunctionUrl', {
      value: fnUrl.url,
      description: 'Public Function URL; curl it to produce one traced invocation.',
    });

    new cdk.CfnOutput(this, 'FunctionName', {
      value: fn.functionName,
      description: 'Lambda function name.',
    });

    new cdk.CfnOutput(this, 'LogGroupName', {
      value: fn.logGroup.logGroupName,
      description: 'CloudWatch log group carrying both the handler and the Collector extension output.',
    });
  }
}
