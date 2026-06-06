import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('DDD Viewer Interactions Tests', () => {
  test('filter Commands page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_commands`);
    await page.fill('input[name="name"]', 'PlaceOrder');
    await page.click('button:has-text("Filter")');
    await expect(page).toHaveURL(/name=PlaceOrder/);
  });

  test('filter Queries page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_queries`);
    await page.fill('input[name="name"]', 'GetOrder');
    await page.click('button:has-text("Filter")');
    await expect(page).toHaveURL(/name=GetOrder/);
  });

  test('filter Events page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_events`);
    await page.fill('input[name="name"]', 'OrderPlaced');
    await page.click('button:has-text("Filter")');
    await expect(page).toHaveURL(/name=OrderPlaced/);
  });

  test('filter Traces page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_traces`);
    await page.fill('input[name="traceID"]', 'test');
    await page.click('button:has-text("Filter")');
    await expect(page).toHaveURL(/traceID=test/);
  });

  test('switch domain tabs', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_domains`);
    const domainTabs = page.locator('.domain-tab');
    const count = await domainTabs.count();
    expect(count).toBeGreaterThan(0);
    
    if (count > 1) {
      await domainTabs.nth(1).click();
      await page.waitForLoadState('networkidle');
    }
  });

  test('empty struct result shows "(empty — no fields)", not "No result metadata"', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_command_types`);

    const resetCard = page.locator('.type-card').filter({ hasText: 'ResetInventoryCommand' });
    await expect(resetCard).toBeVisible();

    const resultSection = resetCard.locator('.type-sub-title').filter({ hasText: 'ResetInventoryResult' });
    await expect(resultSection).toBeVisible();

    const noResultMeta = resetCard.locator('.no-fields').filter({ hasText: 'No result metadata' });
    await expect(noResultMeta).toHaveCount(0);

    const emptyNoFields = resetCard.locator('.no-fields').filter({ hasText: '(empty — no fields)' });
    await expect(emptyNoFields).toBeVisible();
  });

  test('domains page: empty struct result shows "(empty — no fields)"', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_domains?domain=inventory`);

    const resetCard = page.locator('.type-card').filter({ hasText: 'ResetInventoryCommand' });
    await expect(resetCard).toBeVisible();

    const resultSection = resetCard.locator('.type-sub-title').filter({ hasText: 'ResetInventoryResult' });
    await expect(resultSection).toBeVisible();

    const noResultMeta = resetCard.locator('.no-fields').filter({ hasText: 'No result metadata' });
    await expect(noResultMeta).toHaveCount(0);

    const emptyNoFields = resetCard.locator('.no-fields').filter({ hasText: '(empty — no fields)' });
    await expect(emptyNoFields).toBeVisible();
  });
});