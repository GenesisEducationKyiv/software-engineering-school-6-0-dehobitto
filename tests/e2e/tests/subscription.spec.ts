import { test, expect } from '@playwright/test';
import { SubscriptionPage } from '../pages/subscription-page';

const apiKey = process.env.API_KEY ?? 'test-key';

test('form renders with all inputs and submit button', async ({ page }) => {
  const sub = new SubscriptionPage(page);
  await sub.goto();

  await expect(sub.emailInput).toBeVisible();
  await expect(sub.repoInput).toBeVisible();
  await expect(sub.apiKeyInput).toBeVisible();
  await expect(sub.submitButton).toBeVisible();
});

test('subscribes through the real backend and shows duplicate conflict', async ({ page }) => {
  const sub = new SubscriptionPage(page);
  const email = `e2e-${Date.now()}@example.com`;

  await sub.goto();
  await sub.fillForm(email, 'owner/repo', apiKey);
  await sub.submit();

  await expect(sub.successMessage).toBeVisible();
  await expect(sub.successMessage).toContainText('Subscription successful');

  await sub.fillForm(email, 'owner/repo', apiKey);
  await sub.submit();

  await expect(sub.errorMessage).toBeVisible();
  await expect(sub.errorMessage).toContainText('already subscribed');
});

test('shows real backend validation error when repository is not found', async ({ page }) => {
  const sub = new SubscriptionPage(page);

  await sub.goto();
  await sub.fillForm(`missing-${Date.now()}@example.com`, 'owner/missing', apiKey);
  await sub.submit();

  await expect(sub.errorMessage).toBeVisible();
  await expect(sub.errorMessage).toContainText('not found');
});
