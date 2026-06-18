import { defineConfig } from '@playwright/test';

const appUrl = process.env.APP_URL ?? 'http://localhost:4000';
const staticDir = process.env.STATIC_DIR;

export default defineConfig({
  testDir: './tests',
  use: {
    baseURL: appUrl,
  },
  webServer: staticDir
    ? {
        command: `npx serve ${staticDir} -l 4000`,
        port: 4000,
        reuseExistingServer: !process.env.CI,
      }
    : undefined,
});
