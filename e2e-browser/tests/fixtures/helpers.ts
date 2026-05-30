import { Page, expect } from '@playwright/test';

export async function placeOrder(page: Page, opts: {
  userId: string;
  productId: string;
  qty: number;
}): Promise<string> {
  await page.goto(`http://localhost:8080/orders/new`);
  
  await page.fill('input[name="user_id"]', opts.userId);
  await page.fill(`input[name="qty_${opts.productId}"]`, opts.qty.toString());
  
  await page.click('button:has-text("Place Order")');
  
  await page.waitForURL(/\/orders$/);
  
  const orderLink = page.locator('td a:has-text("...")').first();
  await expect(orderLink).toBeVisible();
  
  const href = await orderLink.getAttribute('href');
  const orderId = href?.split('/').pop() ?? '';
  
  return orderId;
}

export async function confirmPayment(page: Page, orderId: string): Promise<void> {
  await page.goto(`http://localhost:8080/orders/${orderId}`);
  await page.click('button:has-text("Confirm Payment")');
  await page.waitForLoadState('networkidle');
}

export async function shipOrder(page: Page, orderId: string): Promise<void> {
  await page.goto(`http://localhost:8080/orders/${orderId}`);
  await page.click('button:has-text("Ship Order")');
  await page.waitForLoadState('networkidle');
}

export async function cancelOrder(page: Page, orderId: string, reason: string): Promise<void> {
  await page.goto(`http://localhost:8080/orders/${orderId}`);
  await page.fill('input[name="reason"]', reason);
  await page.click('button:has-text("Cancel Order")');
  await page.waitForLoadState('networkidle');
}

export async function deleteOrder(page: Page, orderId: string): Promise<void> {
  page.on('dialog', dialog => dialog.accept());
  
  await page.goto(`http://localhost:8080/orders/${orderId}`);
  await page.click('button:has-text("Delete")');
  await page.waitForLoadState('networkidle');
}

export async function getOrderStatus(page: Page, orderId: string): Promise<string> {
  await page.goto(`http://localhost:8080/orders/${orderId}`);
  const badge = page.locator('.badge');
  await expect(badge).toBeVisible();
  const text = await badge.textContent();
  return text?.trim() ?? '';
}

export async function submitJob(page: Page, opts: {
  jobType: string;
  payload?: string;
}): Promise<string> {
  await page.goto('http://localhost:8080/jobs');
  
  await page.fill('input[name="job_type"]', opts.jobType);
  if (opts.payload) {
    await page.fill('textarea[name="payload"]', opts.payload);
  }
  
  await page.click('button:has-text("Submit Job")');
  await page.waitForLoadState('networkidle');
  
  const jobRow = page.locator('table tr').first();
  await expect(jobRow).toBeVisible();
  
  const jobLink = jobRow.locator('td a').first();
  const href = await jobLink.getAttribute('href');
  const jobId = href?.split('/').pop() ?? '';
  
  return jobId;
}

export async function cancelJob(page: Page, jobId: string): Promise<void> {
  await page.goto(`http://localhost:8080/jobs`);
  await page.click(`form[action*="/jobs/${jobId}/cancel"] button`);
  await page.waitForLoadState('networkidle');
}

export async function retryJob(page: Page, jobId: string): Promise<void> {
  await page.goto(`http://localhost:8080/jobs`);
  await page.click(`form[action*="/jobs/${jobId}/retry"] button`);
  await page.waitForLoadState('networkidle');
}