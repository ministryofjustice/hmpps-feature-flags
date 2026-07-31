import { FliptClient } from '@flipt-io/flipt-client-js';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

// The package's type declarations describe the browser build, but Node's
// export condition resolves to the node build, which also has close().
type NodeClient = FliptClient & { close(): void };

import { FliptManagementApi } from './FliptManagementApi';

const baseUrl = process.env.FLIPT_URL ?? 'http://127.0.0.1:8081';
const namespace = { environmentKey: 'test', namespaceKey: 'smoke-test' };

// Tokens and team access are defined in flipt/config/test.yml and
// tests/fixtures/flags/test/smoke-test/access.yml.
const writerApi = new FliptManagementApi({ baseUrl, ...namespace, token: 'smoke-test-writer-token' });
const outsiderApi = new FliptManagementApi({ baseUrl, ...namespace, token: 'smoke-test-outsider-token' });
const anonymousApi = new FliptManagementApi({ baseUrl, ...namespace });

async function pollUntil(condition: () => Promise<boolean>, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    if (await condition()) {
      return;
    }

    await new Promise(resolve => setTimeout(resolve, 500));
  }

  throw new Error(`condition not met within ${timeoutMs}ms`);
}

describe('Flipt smoke test', () => {
  let client: NodeClient;

  beforeAll(async () => {
    client = (await FliptClient.init({ url: baseUrl, environment: 'test', namespace: 'smoke-test' })) as NodeClient;
  });

  afterAll(() => {
    client?.close();
  });

  describe('health', () => {
    it('should report healthy when the instance is up', async () => {
      // Act
      const response = await fetch(`${baseUrl}/health`);

      // Assert
      expect(response.status).toBe(200);
    });
  });

  describe('flag evaluation', () => {
    it('should evaluate an enabled boolean flag as true when no context is supplied', () => {
      // Act
      const result = client.evaluateBoolean({ flagKey: 'always-on', entityId: 'smoke', context: {} });

      // Assert
      expect(result.enabled).toBe(true);
    });

    it('should evaluate a disabled boolean flag as false when no context is supplied', () => {
      // Act
      const result = client.evaluateBoolean({ flagKey: 'always-off', entityId: 'smoke', context: {} });

      // Assert
      expect(result.enabled).toBe(false);
    });

    it('should enable a segmented flag when the context matches the segment', () => {
      // Act
      const result = client.evaluateBoolean({ flagKey: 'segmented-flag', entityId: 'smoke', context: { team: 'smoke' } });

      // Assert
      expect(result.enabled).toBe(true);
    });

    it('should keep a segmented flag disabled when the context does not match the segment', () => {
      // Act
      const result = client.evaluateBoolean({ flagKey: 'segmented-flag', entityId: 'smoke', context: { team: 'other' } });

      // Assert
      expect(result.enabled).toBe(false);
    });

    it('should serve the v2 variant when the context matches the segment', () => {
      // Act
      const result = client.evaluateVariant({ flagKey: 'template-version', entityId: 'smoke', context: { team: 'smoke' } });

      // Assert
      expect(result.variantKey).toBe('v2');
    });

    it('should serve the default variant when the context does not match the segment', () => {
      // Act
      const result = client.evaluateVariant({ flagKey: 'template-version', entityId: 'smoke', context: { team: 'other' } });

      // Assert
      expect(result.variantKey).toBe('v1');
    });
  });

  describe('management API authorization', () => {
    it('should reject flag reads when the token has no namespace access', async () => {
      // Act
      const result = await outsiderApi.getFlag('mutable-flag');

      // Assert
      expect(result.status).toBe(403);
    });

    it('should reject flag updates when the token has no namespace access', async () => {
      // Arrange
      const current = await writerApi.getFlag('mutable-flag');
      expect(current.status).toBe(200);

      // Act
      const result = await outsiderApi.updateFlag({ payload: current.resource!.payload, revision: current.revision! });

      // Assert
      expect(result.status).toBe(403);
    });

    it('should reject requests when no token is supplied', async () => {
      // Act
      const result = await anonymousApi.getFlag('mutable-flag');

      // Assert
      expect(result.status).toBe(401);
    });
  });

  describe('flag mutation', () => {
    it('should reflect a flag change in evaluation when updated via the management API', { timeout: 60_000 }, async () => {
      // Arrange
      const current = await writerApi.getFlag('mutable-flag');
      expect(current.status).toBe(200);
      const flippedEnabled = !current.resource!.payload.enabled;

      // Act
      const update = await writerApi.updateFlag({
        payload: { ...current.resource!.payload, enabled: flippedEnabled },
        revision: current.revision!,
      });

      // Assert
      expect(update.status).toBe(200);

      const after = await writerApi.getFlag('mutable-flag');
      expect(after.resource!.payload.enabled).toBe(flippedEnabled);

      await pollUntil(async () => {
        await client.refresh();

        return client.evaluateBoolean({ flagKey: 'mutable-flag', entityId: 'smoke', context: {} }).enabled === flippedEnabled;
      }, 30_000);
    });
  });
});
