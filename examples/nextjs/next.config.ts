import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Configure CORS headers for trace propagation
  async headers() {
    return [
      {
        source: "/api/:path*",
        headers: [
          {
            key: "Access-Control-Allow-Headers",
            value: "Content-Type, Authorization, traceparent, tracestate",
          },
          {
            key: "Access-Control-Expose-Headers",
            value: "X-Trace-Id",
          },
        ],
      },
    ];
  },
};

export default nextConfig;
