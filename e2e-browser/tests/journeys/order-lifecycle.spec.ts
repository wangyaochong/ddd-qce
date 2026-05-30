import { test, expect } from '@playwright/test';
import { placeOrder, confirmPayment, shipOrder, cancelOrder, deleteOrder } from '../fixtures/helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('Order Lifecycle Journey', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('full order lifecycle: place → pay → ship → events → delete', async ({ page }) => {
    const orderId = await placeOrder(page, { userId: 'test-user', productId: 'laptop', qty: 1 });
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('pending');
    
    await confirmPayment(page, orderId);
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('paid');
    
    await shipOrder(page, orderId);
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('.badge')).toContainText('shipped');
    
    await page.click('a:has-text("View Events")');
    await expect(page.locator('body')).toContainText('OrderPlacedEvent');
    await expect(page.locator('body')).toContainText('PaymentConfirmedEvent');
    await expect(page.locator('body')).toContainText('OrderShippedEvent');
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    page.on('dialog', dialog => dialog.accept());
    await deleteOrder(page, orderId);
    
    await page.goto(`${BASE_URL}/orders/${orderId}`);
    await expect(page.locator('body')).toContainText('order not found');
  });
});