import { chromium, type Browser, type Page } from 'playwright';
import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest';

import { FliptManagementApi } from './FliptManagementApi';

const baseUrl = process.env.FLIPT_URL ?? 'http://127.0.0.1:8081';
const namespace = { environmentKey: 'test', namespaceKey: 'smoke-test' };

// Flipt accepts a static token as its session cookie, which lets the browser
// skip the GitHub login. Tokens are defined in flipt/config/test.yml.
const writerToken = 'smoke-test-writer-token';
const writerApi = new FliptManagementApi({ baseUrl, ...namespace, token: writerToken });

describe('Flipt UI', () => {
  let browser: Browser;
  let page: Page;

  beforeAll(async () => {
    browser = await chromium.launch();
  });

  afterAll(async () => {
    await browser?.close();
  });

  beforeEach(async () => {
    const context = await browser.newContext();
    await context.addCookies([{ name: 'flipt_client_token', value: writerToken, url: baseUrl }]);
    page = await context.newPage();
  });

  it('should show the namespace when a writer opens the UI', async () => {
    // Act
    await page.goto(baseUrl);

    // Assert
    await expect.poll(() => page.title()).toContain('Smoke Test');
  });

  it('should save a flag toggle when a writer updates it in the UI', { timeout: 30_000 }, async () => {
    // Arrange
    const current = await writerApi.getFlag('ui-mutable-flag');
    expect(current.status).toBe(200);
    const flippedEnabled = !current.resource!.payload.enabled;
    await page.goto(baseUrl);
    await expect.poll(() => page.title()).toContain('Smoke Test');

    // Act
    await page.goto(`${baseUrl}/#/namespaces/smoke-test/flags/ui-mutable-flag`);
    await page.getByRole('switch').first().click();
    await page.getByRole('button', { name: 'Update' }).click();

    // Assert
    await expect.poll(async () => (await writerApi.getFlag('ui-mutable-flag')).resource?.payload.enabled).toBe(flippedEnabled);
  });
});
