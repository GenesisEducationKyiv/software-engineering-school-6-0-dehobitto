import { test, expect } from '@playwright/test';
import { SubscriptionPage } from '../pages/subscription-page';

test('form renders with all inputs and submit button', async ({ page }) => {
  const sub = new SubscriptionPage(page);
  await sub.goto();

  await expect(sub.emailInput).toBeVisible();
  await expect(sub.repoInput).toBeVisible();
  await expect(sub.apiKeyInput).toBeVisible();
  await expect(sub.submitButton).toBeVisible();
});

test('shows success message when subscription succeeds', async ({ page }) => {
  await page.route('/api/subscribe', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: 'Subscription successful. Confirmation email sent.' }),
    }),
  );

  const sub = new SubscriptionPage(page);
  await sub.goto();
  await sub.fillForm('user@example.com', 'owner/repo', 'api-key');
  await sub.submit();

  await expect(sub.successMessage).toBeVisible();
  await expect(sub.successMessage).toContainText('Subscription successful');
});

test('shows error message when already subscribed', async ({ page }) => {
  await page.route('/api/subscribe', route =>
    route.fulfill({
      status: 409,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Email already subscribed to this repository' }),
    }),
  );

  const sub = new SubscriptionPage(page);
  await sub.goto();
  await sub.fillForm('user@example.com', 'owner/repo', 'api-key');
  await sub.submit();

  await expect(sub.errorMessage).toBeVisible();
  await expect(sub.errorMessage).toContainText('already subscribed');
});

test('shows error message when repository not found', async ({ page }) => {
  await page.route('/api/subscribe', route =>
    route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Repository not found on GitHub' }),
    }),
  );

  const sub = new SubscriptionPage(page);
  await sub.goto();
  await sub.fillForm('user@example.com', 'owner/nonexistent', 'api-key');
  await sub.submit();

  await expect(sub.errorMessage).toBeVisible();
  await expect(sub.errorMessage).toContainText('not found');
});
