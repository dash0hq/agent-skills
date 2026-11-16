# AWS Lambda fixtures

These fixtures are deliberately uninstrumented AWS Lambda projects that the Lambda scenarios in `evals/custom/scenarios/lambda.go` point the agent at.
The agent adds instrumentation to the infrastructure-as-code. The harness then synthesizes, deploys, and invokes the function in a real AWS account, and reads the resulting spans back from a Dash0 dataset.
Lambda fixtures are not HTTP services, so the service contract in [`../README.md`](../README.md) does not apply to them; the contract below replaces it.

## Contract

Each Lambda fixture is a CDK app with exactly this behavior:

- 1 function whose handler makes 1 outbound call that both layer paths trace, and returns synthetic data (`TEST-`-prefixed identifiers, `example.test` addresses).
  `nodejs-function` uses global `fetch`; `python-function` uses `boto3` (STS `GetCallerIdentity`), because neither Python layer instruments `urllib`.
  `java-function` uses `java.net.http`; `java-aws-function` only calls AWS (STS `GetCallerIdentity`), so the two drive the rules' choice between the `javaagent` and `javawrapper` upstream layers.
  `dotnet-function` uses `HttpClient`; .NET has no auto-instrumentation layer, so its scenarios let the agent change the handler code.
- No OpenTelemetry or Dash0 layers, wrappers, environment variables, or dependencies.
  For a runtime without an auto-instrumentation layer (.NET, Go), the agent instruments the handler in code and must keep its behavior and response.
- X-Ray tracing set to `Active`. The skill tells the agent to resolve this conflict, and the harness rejects a synthesized template that still carries it.
- Exact dependency pins in `package.json`, with a committed `package-lock.json`.

The harness owns these items, and the scenario prompt tells the agent to keep them:

- The stack name and the function name, both read from `EVAL_STACK_NAME`. Every deployed resource matches the `AgentSkillsEval*` scope of the CloudFormation execution policy.
- The `OTEL_RESOURCE_ATTRIBUTES` value from `EVAL_RESOURCE_ATTRIBUTES`. It carries the per-run `test.id` that the read-back query filters on, and the agent appends to it after a comma.
- The permissions boundary from `EVAL_PERMISSIONS_BOUNDARY`, applied to every role in the stack.
- The `dash0OtlpHttpEndpoint`, `dash0OtlpGrpcEndpoint`, `dash0Dataset`, and `dash0TokenSecretArn` stack props. The agent must use them and must not write literals.
- Names of added functions and Lambda layers, which must start with the stack name. The CloudFormation execution policy only allows creating `AgentSkillsEval*` resources, and the harness rejects a layer named otherwise before deploying.

## How the harness runs a Lambda fixture

1. For a compiled runtime, the harness builds the function with the command the scenario prompt names (for Java, `mvn -B -q -DskipTests package` in `lambda/`), on the same scrubbed environment as synthesis. The stack deploys the build output, for Java `lambda/target/function.jar`.
1. `npm install` and `cdk synth` run with the cloud credentials removed from the environment, because the synthesized code is agent-written.
2. `cdk deploy --app cdk.out` and `cdk destroy --app cdk.out` deploy the pre-synthesized assembly, so no agent-written code runs while credentials are present.
3. The harness invokes the function 3 times and polls Dash0 for spans whose `test.id` matches the run.

`node_modules/` and `cdk.out/` are gitignored.
