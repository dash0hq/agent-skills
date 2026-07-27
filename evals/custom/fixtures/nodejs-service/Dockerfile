# Eval fixture: uninstrumented Node.js HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base image.
#
#   docker build -t nodejs-service-eval .
#
# npm install is a no-op while the fixture has zero dependencies; it installs
# whatever an agent adds to package.json while instrumenting.
FROM node:22-alpine
ENV NODE_ENV=production
WORKDIR /app
COPY package*.json ./
RUN npm install --omit=dev
COPY . .
EXPOSE 8080
CMD ["node", "server.js"]
