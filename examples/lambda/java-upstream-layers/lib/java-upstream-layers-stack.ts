import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * Versions of the upstream OpenTelemetry Lambda layers.
 *
 * Source: https://github.com/open-telemetry/opentelemetry-lambda/releases
 * Looked up on 2026-10-29. Latest releases at that point:
 *   - layer-javaagent/0.22.0, published 2026-10-22 (publishes BOTH the javaagent and the
 *     javawrapper layers; there is no separate layer-javawrapper release).
 *   - layer-collector/0.24.0, published 2026-10-22.
 *
 * The layer *name* encodes the version with underscores, not dots: `0_22_0`, not `0.22.0`.
 * Every published layer carries the suffix `:1` -- the Lambda layer version is always 1 because
 * each release publishes a brand-new layer name.
 */
const JAVA_INSTRUMENTATION_LAYER_VERSION = '0_22_0';
const COLLECTOR_LAYER_VERSION = '0_24_0';

/**
 * Which Java instrumentation layer to attach.
 *
 * `javawrapper` is the default per the AWS Lambda deployment rule: the `javaagent` layer loads the
 * full Java agent and adds seconds to cold starts. Flip this to 'javaagent' to get full library
 * auto-instrumentation (including the JDK HTTP client calls this sample makes) at that cost -- see
 * the README section "Which Java layer this uses, and how to switch".
 */
const JAVA_INSTRUMENTATION_LAYER: 'javawrapper' | 'javaagent' = 'javawrapper';

/** Published by the OpenTelemetry project. */
const UPSTREAM_LAYER_ACCOUNT = '184161586896';

/**
 * The function architecture. The Collector layer ARN must match it: an amd64 Collector layer on an
 * arm64 function means the extension binary never starts, the handler keeps succeeding, and
 * nothing is exported.
 */
const ARCHITECTURE = lambda.Architecture.ARM_64;
const COLLECTOR_ARCH_SUFFIX = 'arm64';

export interface JavaUpstreamLayersStackProps extends cdk.StackProps {
  readonly dash0OtlpGrpcEndpoint: string;
  readonly dash0TokenSecretArn: string;
  readonly dash0Dataset: string;
}

export class JavaUpstreamLayersStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: JavaUpstreamLayersStackProps) {
    super(scope, id, props);

    // Layer 1: language instrumentation.
    const instrumentationLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelJavaInstrumentationLayer',
      `arn:aws:lambda:${this.region}:${UPSTREAM_LAYER_ACCOUNT}:layer:opentelemetry-${JAVA_INSTRUMENTATION_LAYER}-${JAVA_INSTRUMENTATION_LAYER_VERSION}:1`,
    );

    // Layer 2: the Collector, which owns the export so the round trip is not billed as handler
    // duration.
    const collectorLayer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'OtelCollectorLayer',
      `arn:aws:lambda:${this.region}:${UPSTREAM_LAYER_ACCOUNT}:layer:opentelemetry-collector-${COLLECTOR_ARCH_SUFFIX}-${COLLECTOR_LAYER_VERSION}:1`,
    );

    // Layer 3: the Collector configuration.
    //
    // CDK zips the *contents* of the asset directory, and `collector-config/` holds exactly one
    // file, so the archive contains `otel-collector-config.yaml` at its root and the file lands at
    // /opt/otel-collector-config.yaml. Verified against the synthesised asset -- see the README.
    const configLayer = new lambda.LayerVersion(this, 'OtelCollectorConfigLayer', {
      layerVersionName: 'otel-collector-config-java-upstream',
      description: 'Collector configuration for the upstream OpenTelemetry Lambda layers',
      code: lambda.Code.fromAsset(path.join(__dirname, '..', 'collector-config')),
      compatibleArchitectures: [ARCHITECTURE],
      compatibleRuntimes: [lambda.Runtime.JAVA_21],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const tokenSecret = secretsmanager.Secret.fromSecretCompleteArn(
      this,
      'Dash0TokenSecret',
      props.dash0TokenSecretArn,
    );

    // The Collector layer reads its auth token from the DASH0_AUTH_TOKEN *environment variable*
    // (${env:DASH0_AUTH_TOKEN} in the configuration). It has no Secrets Manager integration, so
    // the value has to be an env var on the function.
    //
    // This resolves the secret with a CloudFormation dynamic reference. The token never appears in
    // this repository and never appears in the synthesised template -- the template carries the
    // literal string `{{resolve:secretsmanager:<arn>:SecretString:::}}`. CloudFormation resolves
    // it at deploy time. See the README section "How the auth token reaches the Collector" for the
    // residual exposure this leaves.
    const dash0AuthToken = tokenSecret.secretValue.unsafeUnwrap();

    const logGroup = new logs.LogGroup(this, 'TelemetrySampleLogs', {
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const fn = new lambda.Function(this, 'TelemetrySample', {
      runtime: lambda.Runtime.JAVA_21,
      architecture: ARCHITECTURE,
      handler: 'com.example.lambda.TestHandler::handleRequest',
      code: buildJavaArtifact(path.join(__dirname, '..', 'lambda')),
      // A Java cold start on Lambda needs headroom: the JVM starts, the Collector extension
      // registers and parses its configuration, and the handler makes two outbound HTTPS calls.
      // 1024 MB also buys proportionally more vCPU, which shortens JIT warm-up.
      memorySize: 1024,
      timeout: cdk.Duration.seconds(30),
      // X-Ray must be PassThrough: Active makes the X-Ray SDK compete with OpenTelemetry for the
      // trace context and produces broken traces.
      tracing: lambda.Tracing.PASS_THROUGH,
      // CloudFormation renders these in a different order than they are listed here. That is
      // harmless: the three layers write disjoint sets of files into /opt, and layer order only
      // matters when two layers ship the same path.
      layers: [instrumentationLayer, collectorLayer, configLayer],
      logGroup,
      environment: {
        // This is what activates auto-instrumentation. Without it the layers are attached but
        // inert and the function emits nothing. Both Java layers use /opt/otel-handler.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/otel-handler',
        // Points the Collector layer at the YAML from the configuration layer.
        OPENTELEMETRY_COLLECTOR_CONFIG_URI: '/opt/otel-collector-config.yaml',
        OTEL_SERVICE_NAME: 'lambda-telemetry-sample',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=agent-skills-examples,service.version=1.0.0,deployment.environment.name=test',
        // Sample in the Collector, not in the SDK.
        OTEL_TRACES_SAMPLER: 'always_on',
        // Consumed by ${env:...} expansion in otel-collector-config.yaml.
        DASH0_OTLP_GRPC_ENDPOINT: props.dash0OtlpGrpcEndpoint,
        DASH0_DATASET: props.dash0Dataset,
        DASH0_AUTH_TOKEN: dash0AuthToken,
      },
    });

    const functionUrl = fn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'FunctionUrl', {
      value: functionUrl.url,
      description: 'Invoke with: curl "<this value>"',
    });

    new cdk.CfnOutput(this, 'FunctionName', { value: fn.functionName });

    new cdk.CfnOutput(this, 'LogGroupName', {
      value: logGroup.logGroupName,
      description: 'CloudWatch log group carrying the handler logs and the Collector logs',
    });

    new cdk.CfnOutput(this, 'InstrumentationLayerArn', {
      value: instrumentationLayer.layerVersionArn,
    });
  }
}

/**
 * Builds the Java artifact inside a Maven Docker image, so the project needs no local JDK or Maven
 * installation and produces the same layout on every machine.
 *
 * The Lambda Java runtime puts `/var/task/lib/*` on the classpath, so the shaded jar is copied to
 * `lib/function.jar` inside the asset. A bare jar at the root of the asset would not be found.
 */
function buildJavaArtifact(projectDir: string): lambda.AssetCode {
  return lambda.Code.fromAsset(projectDir, {
    // public.ecr.aws/sam/build-java21, which ships a JDK 21 and Maven.
    bundling: {
      image: lambda.Runtime.JAVA_21.bundlingImage,
      user: 'root',
      outputType: cdk.BundlingOutput.NOT_ARCHIVED,
      command: [
        '/bin/sh',
        '-c',
        [
          // Build in a scratch directory so no target/ directory is left in the source tree.
          'cp -r /asset-input /tmp/build',
          'cd /tmp/build',
          'mvn -B -q -DskipTests package',
          'mkdir -p /asset-output/lib',
          'cp target/function.jar /asset-output/lib/function.jar',
        ].join(' && '),
      ],
    },
  });
}
