import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('Dashboard Tests', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('Test Query button shows test result', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Query")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('Test Query');
  });

  test('Test Command button shows test result', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Command")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('Test Command');
  });

  test('Test Event button shows test result', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Event")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('Test Event');
  });

  test('Test QCE Full Lifecycle button shows test result', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test QCE")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('PlaceOrder');
    await expect(page.locator('body')).toContainText('OrderPlaced');
  });

  test('Test Job button shows test result', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Job")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('Test Job');
    await expect(page.locator('body')).toContainText('step');
  });
});