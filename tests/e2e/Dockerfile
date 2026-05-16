FROM mcr.microsoft.com/playwright:v1.52.0-noble

WORKDIR /app/e2e

COPY tests/e2e/package.json ./
RUN npm install

COPY tests/e2e/ ./
COPY internal/static/ ./static/

ENV STATIC_DIR=./static
ENV CI=true

RUN npx playwright install --with-deps chromium

CMD ["npx", "playwright", "test"]
