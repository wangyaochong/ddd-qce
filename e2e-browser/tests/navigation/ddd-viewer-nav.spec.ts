import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('DDD Viewer Navigation Tests', () => {
  test('click Overview nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_commands`);
    await page.click('a[href*="ddd_overview"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_overview/);
    await expect(page.locator('body')).toContainText('DDD');
  });

  test('click Domains nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_domains"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_domains/);
    await expect(page.locator('body')).toContainText('Domain');
  });

  test('click Schema nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_schema/"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_schema\//);
    await expect(page.locator('body')).toContainText('Table');
  });

  test('click Command_Types nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_command_types"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_command_types/);
    await expect(page.locator('body')).toContainText('Command');
  });

  test('click Query_Types nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_query_types"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_query_types/);
    await expect(page.locator('body')).toContainText('Query');
  });

  test('click Event_Types nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_event_types"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_event_types/);
    await expect(page.locator('body')).toContainText('Event');
  });

  test('click Commands nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_commands"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_commands/);
    await expect(page.locator('body')).toContainText('Command');
  });

  test('click Queries nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_queries"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_queries/);
    await expect(page.locator('body')).toContainText('Query');
  });

  test('click Events nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_events"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_events/);
    await expect(page.locator('body')).toContainText('Event');
  });

  test('click Stats nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_stats"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_stats/);
    await expect(page.locator('body')).toContainText('Statistics');
  });

  test('click Jobs nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_jobs"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_jobs/);
    await expect(page.locator('body')).toContainText('Job');
  });

  test('click Traces nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    await page.click('a[href*="ddd_traces"]');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_traces/);
    await expect(page.locator('body')).toContainText('Trace');
  });

  test('click table name link on Overview page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_overview`);
    const tableLink = page.locator('table td a').first();
    await expect(tableLink).toBeVisible();
    await tableLink.click();
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_schema\/.+/);
  });

  test('click Back to Overview link on Schema detail page', async ({ page }) => {
    await page.goto(`${BASE_URL}/api/ddd/ddd_schema/ddd_orders`);
    await expect(page.locator('a:has-text("Back to tables")')).toBeVisible();
    await page.click('a:has-text("Back to tables")');
    await expect(page).toHaveURL(/\/api\/ddd\/ddd_schema\/$/);
  });
});