import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('ExampleApp Navigation Tests', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('click Dashboard nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders`);
    await page.click('a[href="/"]');
    await expect(page).toHaveURL(`${BASE_URL}/`);
    await expect(page.locator('h1')).toContainText('Dashboard');
  });

  test('click Orders nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('a[href="/orders"]');
    await expect(page).toHaveURL(`${BASE_URL}/orders`);
  });

  test('click Inventory nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('a[href="/inventory"]');
    await expect(page).toHaveURL(`${BASE_URL}/inventory`);
  });

  test('click Jobs nav link', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('a[href="/jobs"]');
    await expect(page).toHaveURL(`${BASE_URL}/jobs`);
  });

  test('click DDD Viewer nav link opens new tab', async ({ page, context }) => {
    await page.goto(`${BASE_URL}/`);
    const [newPage] = await Promise.all([
      context.waitForEvent('page'),
      page.click('a[href="/api/ddd/ddd_overview"]')
    ]);
    await newPage.waitForLoadState();
    expect(newPage.url()).toContain('/api/ddd/ddd_overview');
    await expect(newPage.locator('body')).toContainText('DDD');
  });

  test('click Place New Order button on Dashboard', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('a[href="/orders/new"]');
    await expect(page).toHaveURL(`${BASE_URL}/orders/new`);
  });

  test('click View button on order row in Dashboard', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
    await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);
    
    await page.click('td a.btn:has-text("View")');
    await expect(page).toHaveURL(/\/orders\/.+/);
  });

  test('click order ID link in Orders list', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
    await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);
    
    await page.click('td a:first-child');
    await expect(page).toHaveURL(/\/orders\/.+/);
  });

  test('click View Events link on Order Detail', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);

    const orderLink = page.locator('td a').first();
    const href = await orderLink.getAttribute('href');
    const orderId = href?.split('/').pop() ?? '';

    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await page.click('a:has-text("View Events")');
    await expect(page).toHaveURL(new RegExp(`/orders/${orderId}/events`));
  });

  test('click Back to Order Detail on Events page', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
    await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);

    const orderLink = page.locator('td a').first();
    const href = await orderLink.getAttribute('href');
    const orderId = href?.split('/').pop() ?? '';

    await page.goto(`${BASE_URL}/orders/${orderId}/events`);
    await page.click(`a[href="/orders/${orderId}"]`);
    await expect(page).toHaveURL(new RegExp(`/orders/${orderId}$`));
  });

  test('click Cancel link on New Order page', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.click('a:has-text("Cancel")');
    await expect(page).toHaveURL(`${BASE_URL}/orders`);
  });

  test('click Back to Dashboard on Test Result page', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Query")');
    await page.waitForLoadState('networkidle');
    
    await page.click('a:has-text("Back to Dashboard")');
    await expect(page).toHaveURL(`${BASE_URL}/`);
  });
});