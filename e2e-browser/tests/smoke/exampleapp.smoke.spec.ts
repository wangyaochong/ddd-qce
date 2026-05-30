import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('ExampleApp Smoke Tests', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('Dashboard page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await expect(page).toHaveURL(`${BASE_URL}/`);
    await expect(page.locator('h1')).toContainText('Dashboard');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('.stat')).toHaveCount(4);
    await expect(page.locator('button:has-text("Test Query")')).toBeVisible();
    await expect(page.locator('button:has-text("Test Command")')).toBeVisible();
    await expect(page.locator('button:has-text("Test Event")')).toBeVisible();
    await expect(page.locator('button:has-text("Test QCE")')).toBeVisible();
    await expect(page.locator('button:has-text("Test Job")')).toBeVisible();
  });

  test('Orders list page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders`);
    await expect(page).toHaveURL(`${BASE_URL}/orders`);
    await expect(page.locator('h1')).toContainText('Orders');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('a:has-text("New Order")')).toBeVisible();
  });

  test('New Order page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await expect(page).toHaveURL(`${BASE_URL}/orders/new`);
    await expect(page.locator('form[method="POST"]')).toBeVisible();
    await expect(page.locator('input[name="user_id"]')).toBeVisible();
    await expect(page.locator('input[name="qty_laptop"]')).toBeVisible();
    await expect(page.locator('button:has-text("Place Order")')).toBeVisible();
  });

  test('Inventory page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/inventory`);
    await expect(page).toHaveURL(`${BASE_URL}/inventory`);
    await expect(page.locator('h1')).toContainText('Inventory');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('table tr')).toHaveCount(6);
  });

  test('Jobs page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/jobs`);
    await expect(page).toHaveURL(`${BASE_URL}/jobs`);
    await expect(page.locator('h1')).toContainText('Jobs');
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('button:has-text("Test Job")')).toBeVisible();
  });

  test('Order detail page loads', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
    await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);
    
    const orderLink = page.locator('td a').first();
    const href = await orderLink.getAttribute('href');
    const orderId = href?.split('/').pop() ?? '';
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page).toHaveURL(new RegExp(`/orders/${orderId}`));
    await expect(page.locator('.badge')).toBeVisible();
  });

  test('Order events page loads without template error', async ({ page }) => {
    await page.goto(`${BASE_URL}/orders/new`);
    await page.fill('input[name="user_id"]', 'test-user');
    await page.fill('input[name="qty_laptop"]', '1');
    await page.click('button:has-text("Place Order")');
    await page.waitForURL(/\/orders/);
    
    const orderLink = page.locator('td a').first();
    const href = await orderLink.getAttribute('href');
    const orderId = href?.split('/').pop() ?? '';
    
    await page.goto(`${BASE_URL}/orders/${orderId}/events`);
    await expect(page).toHaveURL(new RegExp(`/orders/${orderId}/events`));
    await expect(page.locator('h1')).toContainText('Event Sourcing');
    await expect(page.locator('body')).not.toContainText('template error');
  });
});