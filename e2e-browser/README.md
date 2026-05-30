# Browser E2E Tests for DDD-QCE

End-to-end browser tests using Playwright.

## Prerequisites

1. Start the exampleapp server:
   ```bash
   cd exampleapp && go run .
   ```

2. Install dependencies:
   ```bash
   cd e2e-browser && npm install
   ```

3. Install Playwright browsers:
   ```bash
   npx playwright install chromium
   ```

## Run Tests

```bash
npm test                  # Run all tests
npm run test:headed       # Run with browser visible
npm run test:debug        # Debug mode
npm run test:navigation   # Run navigation tests only
npm run test:order-flow   # Run order flow tests only
npm run test:ddd-viewer   # Run DDD Viewer tests only
npm run test:trace-multi-span  # Run multi-span trace tests only
```

## Environment Variables

- `BASE_URL`: Override the base URL (default: `http://localhost:8080`)

Example:
```bash
BASE_URL=http://example.com npm test
```

## Test Files

| File | Description |
|------|-------------|
| `navigation.spec.ts` | Tests navigation bar links work correctly |
| `order-flow.spec.ts` | Tests order creation page flow |
| `ddd-viewer.spec.ts` | Tests DDD Viewer pages render correctly |
| `trace-multi-span.spec.ts` | Tests that QCE Full Lifecycle generates multi-span traces with parent-child relationships |

## Migration Notes

This test suite was migrated from Puppeteer to Playwright. Key changes:
- Uses `@playwright/test` instead of `puppeteer`
- TypeScript test files (`.spec.ts`) instead of JavaScript (`.spec.js`)
- Playwright's built-in assertions (`expect`) instead of manual result tracking
- Playwright config file (`playwright.config.ts`) replaces manual browser launch