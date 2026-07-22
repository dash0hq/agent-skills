# Eval fixture: uninstrumented static web app (see evals/fixtures/README.md).
# Deterministic build: pinned base image.
#
#   docker build -t browser-service-eval .
#
# npm install is a no-op while the fixture has zero dependencies; it installs
# whatever an agent adds to package.json while instrumenting (for example a
# bundler that builds the page instrumentation into static/).
FROM node:22-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 8080
CMD ["node", "server.js"]
