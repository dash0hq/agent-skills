// The browser instrumentation scenario. It differs from the other SDK
// scenarios in every fragile way at once: the telemetry source is a web page
// (not the fixture's server process), exporter configuration reaches the page
// through the fixture-served /env.js instead of process environment
// variables, exports travel OTLP/HTTP through the relay's CORS-answering
// endpoint, and traffic is a headless Chromium container loading the page.
//
// This is the most fragile scenario in the matrix and the natural first
// quarantine candidate (see evals/custom/quarantine.yaml and the plan's risk
// register): if it flakes, quarantine it rather than letting it block the PR
// gate.
package scenarios

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

func init() {
	registerScenarios(BrowserHTTP())
}

// BrowserHTTPID is the browser instrumentation scenario: page spans received
// over OTLP/HTTP through the relay.
const BrowserHTTPID = "instr-browser-http"

// BrowserServiceName is the service.name the browser scenario requires.
const BrowserServiceName = "browser-service-eval"

// browserChromiumImage is the pinned headless Chromium image that loads the
// page as the traffic driver.
const browserChromiumImage = "zenika/alpine-chrome:124"

// browserPageLoads is how many times Chromium loads the page; at least 1 load
// must succeed.
const browserPageLoads = 2

// BrowserHTTP is the browser instrumentation scenario. Assertions target
// spans produced by the page under the required service.name, including a
// span for the page's /checkout-data fetch; server-side spans do not satisfy
// them because the fixture's server stays uninstrumented.
func BrowserHTTP() harness.Scenario {
	return harness.Scenario{
		ID:          BrowserHTTPID,
		Skill:       harness.SkillInstrumentation,
		RuleFiles:   []string{"skills/otel-instrumentation/rules/sdks/browser.md"},
		FixturePath: "evals/custom/fixtures/browser-service",
		Prompt: `Instrument the static web application in the current directory with OpenTelemetry browser instrumentation, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation). The page (static/index.html plus static/app.js) is served by server.js and fetches /checkout-data on load; the telemetry must come from the page running in the browser, not from server.js.

Instrumentation goals:
- Export the page's traces via OTLP over HTTP (http/protobuf or http/json) directly from the browser.
- Set the service name resource attribute to "` + BrowserServiceName + `".
- Capture the page load and the page's fetch of /checkout-data so at least 1 span references /checkout-data.

Requirements:
- Browsers cannot read process environment variables. The server already exposes its EVAL_-prefixed runtime configuration to the page as window.__EVAL_ENV__ via GET /env.js: use EVAL_OTLP_ENDPOINT as the OTLP base URL (append the /v1/traces signal path), send "Authorization: Bearer <EVAL_OTLP_TOKEN>" on exports, and parse EVAL_RESOURCE_ATTRIBUTES (comma-separated key=value pairs) into resource attributes on the page's telemetry. Do not hardcode endpoints, tokens, or resource-attribute values.
- The page is loaded by a short-lived headless browser, so spans must export within a few seconds of the page load: configure a short batch delay or force a flush once the page has loaded and the /checkout-data fetch has settled.
- Preserve the existing behavior: server.js must keep serving the page, /env.js, GET /checkout, and GET /checkout-data, keep calling the URL from the DOWNSTREAM_URL environment variable, and keep honoring the PORT environment variable.
- The Dockerfile must keep building successfully; if the instrumentation needs a build step (for example a bundler), add it to the image build and serve the result from static/.`,
		Timeout:          heavyScenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertBrowserSpans(BrowserServiceName),
	}
}

// fixtureHooks selects the Docker-backed hooks for one scenario: the browser
// scenario drives traffic with a headless Chromium page load, every other
// scenario uses the HTTP probe topology of DockerFixture.Hooks.
func fixtureHooks(fix *DockerFixture, sc harness.Scenario) harness.FixtureHooks {
	if sc.ID == BrowserHTTPID {
		return fix.BrowserHooks()
	}
	return fix.Hooks()
}

// assertBrowserSpans builds the browser assertion: spans under the required
// service.name reached the sink (necessarily over the relay's OTLP/HTTP path,
// the page's only route), and at least 1 of them references the page's
// /checkout-data fetch, proving the telemetry came from page activity.
func assertBrowserSpans(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		all := sink.Traces(t)
		svc := all.WithResourceAttribute("service.name", serviceName)
		if svc.Len() == 0 {
			return fmt.Errorf("no spans with resource attribute service.name=%q at the sink (%d spans total, names: %v)", serviceName, all.Len(), all.Names())
		}
		for _, sv := range svc.Spans() {
			if spanReferencesPath(sv.Span, "/checkout-data") {
				return nil
			}
		}
		return fmt.Errorf("no span with service.name=%q references the page's /checkout-data fetch (span names: %v)", serviceName, svc.Names())
	}
}

// BrowserHooks returns fixture hooks for the browser scenario: the shared
// workspace image build plus a run step whose traffic driver is a headless
// Chromium container loading the page.
func (d *DockerFixture) BrowserHooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: d.build, Run: d.runBrowser}
}

// runBrowser starts the browser-scenario topology for one attempt. It mirrors
// DockerFixture.run — internal network, host proxy, dual-homed relay
// container, stub downstream, fixture — and then differs in traffic: after a
// single readiness probe of GET /checkout, a headless Chromium container on
// the internal network loads the page, which is what produces the telemetry.
// The fixture container additionally receives EVAL_RESOURCE_ATTRIBUTES, which
// server.js forwards to the page via /env.js (browsers cannot read process
// environment variables).
func (d *DockerFixture) runBrowser(ctx context.Context, _ string, env map[string]string) error {
	d.mu.Lock()
	image := d.currentImage
	d.mu.Unlock()
	if image == "" {
		return fmt.Errorf("docker run: no fixture image built for this attempt")
	}
	if err := d.ensureInfraImages(ctx); err != nil {
		return err
	}

	// The page-facing copy of the resource attributes, forwarded by server.js
	// through /env.js (browsers cannot read process environment variables).
	topo, err := d.buildContainerTopology(ctx, image, env, map[string]string{
		"EVAL_RESOURCE_ATTRIBUTES": env["OTEL_RESOURCE_ATTRIBUTES"],
	})
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.currentApp = topo.appName
	d.mu.Unlock()

	// Readiness: 1 successful GET /checkout proves the server is up before
	// Chromium spends its page-load budget.
	checkoutURL := fmt.Sprintf("http://%s:%s/checkout", appAlias, appPort)
	if out, err := docker(ctx, "run", "--rm", "--network", topo.network,
		helperImage, "probe", checkoutURL, "1"); err != nil {
		logs, _ := docker(context.Background(), "logs", topo.appName)
		return fmt.Errorf("readiness probe failed against %s: %w\n%s\nfixture logs:\n%s", checkoutURL, err, out, logs)
	}

	// Traffic: headless Chromium loads the page; the page's instrumentation
	// is what exports telemetry. The virtual-time budget gives the page's
	// fetches and the exporter time to run before Chromium exits.
	pageURL := fmt.Sprintf("http://%s:%s/", appAlias, appPort)
	succeeded := 0
	var lastErr error
	for i := 0; i < browserPageLoads; i++ {
		out, err := docker(ctx, "run", "--rm", "--network", topo.network,
			"--entrypoint", "chromium-browser", browserChromiumImage,
			"--headless", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
			"--disable-software-rasterizer", "--virtual-time-budget=20000",
			"--dump-dom", pageURL)
		if err == nil {
			succeeded++
			continue
		}
		lastErr = fmt.Errorf("chromium page load %d/%d failed: %w\n%s", i+1, browserPageLoads, err, out)
	}
	if succeeded == 0 {
		logs, _ := docker(context.Background(), "logs", topo.appName)
		return fmt.Errorf("headless Chromium traffic failed against %s: %w\nfixture logs:\n%s", pageURL, lastErr, logs)
	}
	return nil
}
