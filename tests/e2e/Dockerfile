FROM mcr.microsoft.com/playwright:v1.52.0-noble

WORKDIR /app/e2e

COPY tests/e2e/package.json tests/e2e/package-lock.json ./
RUN npm ci

COPY tests/e2e/ ./
COPY services/subscription-api/static/ ./static/

ENV STATIC_DIR=./static
ENV CI=true

RUN npx playwright install --with-deps chromium

CMD ["npx", "playwright", "test"]
