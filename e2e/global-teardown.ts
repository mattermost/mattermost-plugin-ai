import { FullConfig } from '@playwright/test';

async function globalTeardown(config: FullConfig) {
  // Clean up resources after all tests
  // - Close shared browser instances
  // - Remove temporary files
  // - Reset environment
  console.log('Global teardown complete');
}

export default globalTeardown;
