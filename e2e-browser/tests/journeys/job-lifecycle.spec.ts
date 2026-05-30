import { test, expect } from '@playwright/test';
import { seedJob } from '../fixtures/reset';
import { submitJob, cancelJob, retryJob } from '../fixtures/helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('Job Lifecycle Journey', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.post(`${BASE_URL}/admin/reset`);
    expect(res.status()).toBe(200);
  });

  test('can submit a new job', async ({ page }) => {
    await page.goto(`${BASE_URL}/jobs`);
    await page.fill('input[name="job_type"]', 'test-job');
    await page.fill('textarea[name="payload"]', '{"test":true}');
    await page.click('button:has-text("Submit Job")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('table tr')).toHaveCount(2);
  });

  test('Test Job button shows test result page', async ({ page }) => {
    await page.goto(`${BASE_URL}/`);
    await page.click('button:has-text("Test Job")');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('body')).toContainText('Test Job');
    await expect(page.locator('body')).toContainText('step');
  });

  test('can cancel a running job', async ({ page, request }) => {
    const jobId = await seedJob(request, 'running');
    
    await page.goto(`${BASE_URL}/jobs`);
    await page.click(`form[action*="/jobs/${jobId}/cancel"] button`);
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator(`text=${jobId}`)).toBeVisible();
    await expect(page.locator('.badge-failed, .badge-cancelled')).toBeVisible();
  });

  test('can retry a failed job', async ({ page, request }) => {
    const jobId = await seedJob(request, 'failed');
    
    await page.goto(`${BASE_URL}/jobs`);
    await expect(page.locator(`text=${jobId}`)).toBeVisible();
    
    await page.click(`form[action*="/jobs/${jobId}/retry"] button`);
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.badge-running')).toBeVisible();
  });
});