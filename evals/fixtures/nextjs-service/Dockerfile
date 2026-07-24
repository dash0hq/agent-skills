# Eval fixture: uninstrumented Next.js app-router app (see
# evals/fixtures/README.md). Deterministic build: pinned base image, exact
# dependency versions in package.json. The listen port comes from PORT.
#
#   docker build -t nextjs-service-eval .
FROM node:22-alpine
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build
EXPOSE 8080
CMD ["sh", "-c", "./node_modules/.bin/next start -p ${PORT:-8080}"]
