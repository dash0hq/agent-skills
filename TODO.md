* How to record exceptions
* RED metrics aggregation
* Logs sampling ?!?
* Be careful about resource attributes, especially k8s.* and deployment.* ones, and make sure those are not advised as part of changing the app, but to deploy it
* How to add the OTel API and add attributes to existing spans
* Integrate injector packages?
* Move testing to language-specific files
* Add planning guidance to add relevant tests are every phase

# Go

  OTEL_LOGS_EXPORTER=otlp silently does nothing: The skill initializes an OTel log provider in initTelemetry but never shows how to bridge slog to it. The log provider sits idle and zero log records reach the collector. This is different from Python, where the auto-instrumentation bridges the stdlib logging module automatically. In Go, the bridge is manual and the skill doesn't document it.
[3:06 PM]Moving to Java and then Ruby
Julia Furst Morgado  [4:24 PM]


# .NET

  Apple Silicon blocks the documented installation path: 
  The dotnet.md skill only documents the zero-code install script. That script explicitly doesn't support Apple Silicon, and the troubleshooting section just says "use Linux or a container" with no
  further guidance. An agent following the skill on an Apple Silicon Mac is completely stuck. I had to switch to the NuGet SDK approach, which the skill doesn't document at all.

  The NuGet workaround breaks the env var contract: 
  Because I was forced onto NuGet packages, the env var table in the skill stopped applying. OTEL_TRACES_EXPORTER=console had no effect — when .AddOtlpExporter() is wired in code, the env var doesn't override it. The skill doesn't mention this difference, so a developer following the NuGet path would have no idea why switching to the console exporter for local debugging doesn't work.

# Adding business attributes

  Activity.Current pattern not shown for server span enrichment: 
  The skill's custom spans section only covers creating child spans with ActivitySource.StartActivity(). It doesn't show the pattern for adding business attributes to auto-instrumented SERVER spans
  (Activity.Current?.SetTag("order.id", id)). I used this successfully and it works, it's just not in the skill.
