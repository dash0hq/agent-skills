# Eval fixture: uninstrumented PHP HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base image; Composer is available for the
# dependencies an agent adds while instrumenting. The listen port comes from
# PORT.
#
#   docker build -t php-service-eval .
FROM php:8.3-cli
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer
WORKDIR /app
COPY composer.json ./
RUN composer install --no-interaction --no-progress
COPY . .
EXPOSE 8080
CMD ["sh", "-c", "php -S 0.0.0.0:${PORT:-8080} index.php"]
