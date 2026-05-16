import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  use: {
    baseURL: 'http://localhost:4000',
  },
  webServer: {
    command: `npx serve ${process.env.STATIC_DIR ?? '../../internal/static'} -l 4000`,
    port: 4000,
    reuseExistingServer: !process.env.CI,
  },
});
