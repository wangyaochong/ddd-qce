import { test, expect } from '@playwright/test';
import { placeOrder, cancelOrder } from '../fixtures/helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('Order Cancel Journey', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('pending order can be cancelled', async ({ page }) => {
    const orderId = await placeOrder(page, { userId: 'test-user', productId: 'laptop', qty: 1 });
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('pending');
    
    await cancelOrder(page, orderId, 'Changed my mind');
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('cancelled');
    await expect(page.locator('body')).toContainText('Changed my mind');
  });

  test('paid order can be cancelled', async ({ page }) => {
    const orderId = await placeOrder(page, { userId: 'test-user', productId: 'laptop', qty: 1 });
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await page.click('button:has-text("Confirm Payment")');
    await page.waitForLoadState('networkidle');
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('paid');
    
    await cancelOrder(page, orderId, 'Customer requested refund');
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('cancelled');
  });

  test('shipped order cannot be cancelled', async ({ page }) => {
    const orderId = await placeOrder(page, { userId: 'test-user', productId: 'laptop', qty: 1 });
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await page.click('button:has-text("Confirm Payment")');
    await page.waitForLoadState('networkidle');
    await page.click('button:has-text("Ship Order")');
    await page.waitForLoadState('networkidle');
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('shipped');
    
    const cancelButton = page.locator('button:has-text("Cancel Order")');
    await expect(cancelButton).not.toBeVisible();
  });
});