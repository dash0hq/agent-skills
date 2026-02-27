"use client";

import { useState } from "react";

interface ApiResponse {
  success: boolean;
  data?: {
    id?: string;
    name: string;
    value: number;
    processed?: boolean;
    result?: number;
    timestamp?: string;
    createdAt?: string;
  };
  _trace?: {
    traceId: string;
    spanId: string;
  };
  error?: string;
}

export default function Home() {
  const [response, setResponse] = useState<ApiResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [traceId, setTraceId] = useState<string | null>(null);
  const [formData, setFormData] = useState({ name: "", value: "" });

  const fetchData = async (includeProcessing = false) => {
    setLoading(true);
    try {
      const res = await fetch(
        `/api/demo?id=item-${Date.now()}&process=${includeProcessing}`
      );
      const data: ApiResponse = await res.json();
      setResponse(data);
      setTraceId(res.headers.get("X-Trace-Id"));
    } catch {
      setResponse({ success: false, error: "Network error" });
    } finally {
      setLoading(false);
    }
  };

  const postData = async () => {
    if (!formData.name || !formData.value) return;

    setLoading(true);
    try {
      const res = await fetch("/api/demo", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: formData.name,
          value: parseFloat(formData.value),
        }),
      });
      const data: ApiResponse = await res.json();
      setResponse(data);
      setTraceId(res.headers.get("X-Trace-Id"));
    } catch {
      setResponse({ success: false, error: "Network error" });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-8">
      <div className="max-w-4xl mx-auto space-y-8">
        <header className="text-center space-y-2">
          <h1 className="text-4xl font-bold text-white">
            OpenTelemetry Demo
          </h1>
          <p className="text-gray-400">
            Next.js with traces, metrics, logs, and server-client correlation
          </p>
        </header>

        {/* GET Request Section */}
        <section className="bg-gray-900 rounded-lg p-6 space-y-4">
          <h2 className="text-xl font-semibold text-white">GET Request</h2>
          <p className="text-gray-400 text-sm">
            Fetches simulated data with optional processing. Creates spans for
            database fetch and data processing operations.
          </p>
          <div className="flex gap-4">
            <button
              onClick={() => fetchData(false)}
              disabled={loading}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-lg transition-colors"
            >
              Fetch Data
            </button>
            <button
              onClick={() => fetchData(true)}
              disabled={loading}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 rounded-lg transition-colors"
            >
              Fetch + Process
            </button>
          </div>
        </section>

        {/* POST Request Section */}
        <section className="bg-gray-900 rounded-lg p-6 space-y-4">
          <h2 className="text-xl font-semibold text-white">POST Request</h2>
          <p className="text-gray-400 text-sm">
            Submits data for processing. Demonstrates request validation and
            structured logging.
          </p>
          <div className="flex gap-4 items-end">
            <div className="space-y-1">
              <label className="text-sm text-gray-400">Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="Enter name"
                className="px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg focus:border-blue-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1">
              <label className="text-sm text-gray-400">Value</label>
              <input
                type="number"
                value={formData.value}
                onChange={(e) =>
                  setFormData({ ...formData, value: e.target.value })
                }
                placeholder="Enter value"
                className="px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg focus:border-blue-500 focus:outline-none"
              />
            </div>
            <button
              onClick={postData}
              disabled={loading || !formData.name || !formData.value}
              className="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:opacity-50 rounded-lg transition-colors"
            >
              Submit
            </button>
          </div>
        </section>

        {/* Response Section */}
        {(response || loading) && (
          <section className="bg-gray-900 rounded-lg p-6 space-y-4">
            <h2 className="text-xl font-semibold text-white">Response</h2>

            {loading && (
              <div className="flex items-center gap-2 text-gray-400">
                <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                Loading...
              </div>
            )}

            {response && !loading && (
              <div className="space-y-4">
                {/* Status */}
                <div className="flex items-center gap-2">
                  <span
                    className={`px-2 py-1 rounded text-sm ${
                      response.success
                        ? "bg-green-900 text-green-300"
                        : "bg-red-900 text-red-300"
                    }`}
                  >
                    {response.success ? "Success" : "Error"}
                  </span>
                </div>

                {/* Trace ID for correlation */}
                {traceId && (
                  <div className="bg-gray-800 rounded p-3 space-y-1">
                    <p className="text-xs text-gray-500 uppercase tracking-wide">
                      Trace ID (Server-Client Correlation)
                    </p>
                    <code className="text-yellow-400 text-sm break-all">
                      {traceId}
                    </code>
                  </div>
                )}

                {/* Data */}
                <div className="bg-gray-800 rounded p-3">
                  <p className="text-xs text-gray-500 uppercase tracking-wide mb-2">
                    Response Data
                  </p>
                  <pre className="text-sm text-gray-300 overflow-auto">
                    {JSON.stringify(response, null, 2)}
                  </pre>
                </div>
              </div>
            )}
          </section>
        )}

        {/* Telemetry Info */}
        <section className="bg-gray-800/50 rounded-lg p-6 space-y-4">
          <h2 className="text-lg font-semibold text-white">
            Telemetry Signals
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div className="space-y-2">
              <h3 className="font-medium text-blue-400">Traces/Spans</h3>
              <ul className="text-gray-400 space-y-1">
                <li>- demo.api.get / demo.api.post</li>
                <li>- demo.db.fetch</li>
                <li>- demo.process_data</li>
              </ul>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium text-green-400">Metrics</h3>
              <ul className="text-gray-400 space-y-1">
                <li>- demo.requests (counter)</li>
                <li>- demo.request.duration (histogram)</li>
                <li>- demo.requests.active (gauge)</li>
              </ul>
            </div>
            <div className="space-y-2">
              <h3 className="font-medium text-purple-400">Logs</h3>
              <ul className="text-gray-400 space-y-1">
                <li>- api.request.received</li>
                <li>- db.query.executed</li>
                <li>- data.processed</li>
                <li>- api.request.completed</li>
              </ul>
            </div>
          </div>
        </section>

        {/* Setup Info */}
        <section className="text-center text-gray-500 text-sm space-y-2">
          <p>
            To view telemetry, run an OTel Collector or use a backend like Jaeger, Grafana, or Dash0.
          </p>
          <p>
            <code className="bg-gray-800 px-2 py-1 rounded">
              OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 npm run dev
            </code>
          </p>
        </section>
      </div>
    </div>
  );
}
