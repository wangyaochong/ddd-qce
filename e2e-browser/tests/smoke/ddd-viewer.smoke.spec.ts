import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

test.describe('DDD Viewer Smoke Tests', () => {
  const dddPages = [
    { name: 'Overview', path: '/api/ddd/ddd_overview', expectContent: 'DDD' },
    { name: 'Domains', path: '/api/ddd/ddd_domains', expectContent: 'Domain' },
    { name: 'Schema', path: '/api/ddd/ddd_schema/', expectContent: 'Table' },
    { name: 'Command_Types', path: '/api/ddd/ddd_command_types', expectContent: 'Command' },
    { name: 'Query_Types', path: '/api/ddd/ddd_query_types', expectContent: 'Query' },
    { name: 'Event_Types', path: '/api/ddd/ddd_event_types', expectContent: 'Event' },
    { name: 'Commands', path: '/api/ddd/ddd_commands', expectContent: 'Command' },
    { name: 'Queries', path: '/api/ddd/ddd_queries', expectContent: 'Query' },
    { name: 'Events', path: '/api/ddd/ddd_events', expectContent: 'Event' },
    { name: 'Stats', path: '/api/ddd/ddd_stats', expectContent: 'Statistics' },
    { name: 'Jobs', path: '/api/ddd/ddd_jobs', expectContent: 'Job' },
    { name: 'Traces', path: '/api/ddd/ddd_traces', expectContent: 'Trace' },
  ];

  for (const { name, path, expectContent } of dddPages) {
    test(`${name} page loads correctly`, async ({ page }) => {
      await page.goto(`${BASE_URL}${path}`);
      await expect(page).toHaveURL(`${BASE_URL}${path}`);
      await expect(page.locator('body')).toContainText(expectContent);
      await expect(page.locator('nav')).toBeVisible();
    });
  }
});