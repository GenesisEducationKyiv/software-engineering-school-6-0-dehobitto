import { type Locator, type Page } from '@playwright/test';

export class SubscriptionPage {
  readonly emailInput: Locator;
  readonly repoInput: Locator;
  readonly apiKeyInput: Locator;
  readonly submitButton: Locator;
  readonly successMessage: Locator;
  readonly errorMessage: Locator;

  constructor(private readonly page: Page) {
    this.emailInput = page.locator('#email');
    this.repoInput = page.locator('#repo');
    this.apiKeyInput = page.locator('#apikey');
    this.submitButton = page.locator('button[type="submit"]');
    this.successMessage = page.locator('#msg.ok');
    this.errorMessage = page.locator('#msg.err');
  }

  async goto() {
    await this.page.goto('/');
  }

  async fillForm(email: string, repo: string, apiKey = '') {
    await this.emailInput.fill(email);
    await this.repoInput.fill(repo);
    await this.apiKeyInput.fill(apiKey);
  }

  async submit() {
    await this.submitButton.click();
  }
}
