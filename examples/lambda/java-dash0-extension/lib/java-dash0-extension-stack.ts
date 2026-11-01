import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * Version of the Dash0 Lambda Extension layer.
 *
 * Source: https://github.com/dash0hq/dash0-lambda-extension/releases
 * Latest release at lookup time: v27, published 2026-10-27. Looked up on 2026-10-29.
 *
 * The release tag and the Lambda layer version are the same number, so `v27` is published as
 * `...:layer:dash0-extension-java:27`.
 */
const DASH0_EXTENSION_LAYER_VERSION = 27;

/** Published by Dash0; see the layer ARN table in the AWS Lambda deployment rule. */
const DASH0_LAYER_ACCOUNT = '115813213817';

export interface JavaDash0ExtensionStackProps extends cdk.StackProps {
  readonly dash0Endpoint: string;
  readonly dash0TokenSecretArn: string;
  readonly dash0Dataset: string;
}

export class JavaDash0ExtensionStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: JavaDash0ExtensionStackProps) {
    super(scope, id, props);

    const tokenSecret = secretsmanager.Secret.fromSecretCompleteArn(
      this,
      'Dash0TokenSecret',
      props.dash0TokenSecretArn,
    );

    // One layer only. The Dash0 extension replaces the three upstream layers of path A, so the
    // upstream Collector layer must NOT be attached alongside it.
    const dash0Layer = lambda.LayerVersion.fromLayerVersionArn(
      this,
      'Dash0ExtensionLayer',
      `arn:aws:lambda:${this.region}:${DASH0_LAYER_ACCOUNT}:layer:dash0-extension-java:${DASH0_EXTENSION_LAYER_VERSION}`,
    );

    const logGroup = new logs.LogGroup(this, 'TelemetrySampleLogs', {
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const fn = new lambda.Function(this, 'TelemetrySample', {
      runtime: lambda.Runtime.JAVA_21,
      architecture: lambda.Architecture.ARM_64,
      handler: 'com.example.lambda.TestHandler::handleRequest',
      code: buildJavaArtifact(path.join(__dirname, '..', 'lambda')),
      // A Java cold start on Lambda needs headroom: the JVM starts, the extension registers, and
      // the handler makes two outbound HTTPS calls. 1024 MB also buys proportionally more vCPU.
      memorySize: 1024,
      timeout: cdk.Duration.seconds(30),
      // X-Ray must be PassThrough: Active makes the X-Ray SDK compete with OpenTelemetry for the
      // trace context and produces broken traces.
      tracing: lambda.Tracing.PASS_THROUGH,
      layers: [dash0Layer],
      logGroup,
      environment: {
        // Activates auto-instrumentation. Without it the layer is attached but only logs are
        // collected -- no traces.
        AWS_LAMBDA_EXEC_WRAPPER: '/opt/wrapper',
        DASH0_ENDPOINT: props.dash0Endpoint,
        DASH0_DATASET: props.dash0Dataset,
        // Never DASH0_TOKEN: a literal token in the function configuration is readable through
        // lambda:GetFunctionConfiguration and ends up in the IaC source and in CloudTrail.
        DASH0_TOKEN_SECRET_ARN: tokenSecret.secretArn,
        // DASH0_TOKEN_SECRET_KEY is deliberately unset: the secret holds the token as a plain
        // string, not as a JSON object.
        OTEL_SERVICE_NAME: 'lambda-telemetry-sample',
        OTEL_RESOURCE_ATTRIBUTES:
          'service.namespace=agent-skills-examples,service.version=1.0.0,deployment.environment.name=test',
      },
    });

    // The extension reads the secret at startup, so the execution role needs
    // secretsmanager:GetSecretValue on it.
    tokenSecret.grantRead(fn);

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
      description: 'CloudWatch log group carrying both the handler logs and the extension logs',
    });
  }
}

/**
 * Builds the Java artifact inside a Maven Docker image, so the project needs no local JDK or Maven
 * installation and produces a byte-identical layout regardless of the developer machine.
 *
 * The Lambda Java runtime puts `/var/task/lib/*` on the classpath, so the shaded jar is copied to
 * `lib/function.jar` inside the asset. A bare jar at the root of the asset would not be found.
 */
function buildJavaArtifact(projectDir: string): lambda.AssetCode {
  return lambda.Code.fromAsset(projectDir, {
    bundling: {
      // public.ecr.aws/sam/build-java21, which ships a JDK 21 and Maven.
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
