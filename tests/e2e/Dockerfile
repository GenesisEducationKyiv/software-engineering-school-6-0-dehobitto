FROM mcr.microsoft.com/playwright:v1.60.0-noble

WORKDIR /app/e2e

COPY tests/e2e/package*.json ./
RUN npm ci

COPY tests/e2e/ ./
COPY services/subscription-api/static/ ./static/

ENV STATIC_DIR=./static
ENV CI=true

CMD ["npx", "playwright", "test"]
