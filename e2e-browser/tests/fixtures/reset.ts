import { APIRequestContext, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

export async function resetDatabase(request: APIRequestContext): Promise<void> {
  const res = await request.post(`${BASE_URL}/admin/reset`);
  if (res.status() !== 200) {
    const body = await res.text();
    throw new Error(`resetDatabase failed: ${res.status()} ${body}`);
  }
  expect(res.status()).toBe(200);
}

export async function seedJob(request: APIRequestContext, status: 'running' | 'failed'): Promise<string> {
  const res = await request.post(`${BASE_URL}/test/seed-job?status=${status}`);
  expect(res.status()).toBe(200);
  const body = await res.json();
  return body.jobId;
}