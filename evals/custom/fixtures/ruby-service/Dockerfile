# Eval fixture: uninstrumented Ruby HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base image; the listen port comes from PORT.
#
#   docker build -t ruby-service-eval .
FROM ruby:3.4-slim
WORKDIR /app
COPY Gemfile ./
RUN bundle install
COPY . .
EXPOSE 8080
CMD ["sh", "-c", "bundle exec rackup --server webrick --host 0.0.0.0 --port ${PORT:-8080}"]
