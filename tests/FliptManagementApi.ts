export enum FlagType {
  BOOLEAN = 'BOOLEAN_FLAG_TYPE',
  VARIANT = 'VARIANT_FLAG_TYPE',
}

export interface FlagPayload {
  '@type': 'flipt.core.Flag';
  key: string;
  type: FlagType;
  name?: string;
  description?: string;
  enabled: boolean;
}

export interface FlagResource {
  namespaceKey: string;
  key: string;
  payload: FlagPayload;
}

export interface FlagReadResult {
  status: number;
  resource?: FlagResource;
  revision?: string;
}

export interface FlagUpdateResult {
  status: number;
  revision?: string;
}

interface FliptManagementApiOptions {
  baseUrl: string;
  environmentKey: string;
  namespaceKey: string;
  token?: string;
}

interface ResourceResponseBody {
  resource: FlagResource;
  revision: string;
}

/**
 * Minimal client for Flipt's v2 management API (the environments/resources
 * endpoints), covering just what the smoke tests need: reading a flag and
 * updating it. Expected failures (401/403/404) come back as a bare status
 * rather than a throw so tests can assert on them.
 */
export class FliptManagementApi {
  constructor(private readonly options: FliptManagementApiOptions) {}

  async getFlag(flagKey: string): Promise<FlagReadResult> {
    const response = await this.request({ method: 'GET', url: `${this.resourcesUrl()}/flipt.core.Flag/${flagKey}` });

    if (!response.ok) {
      return { status: response.status };
    }

    const body = (await response.json()) as ResourceResponseBody;

    return { status: response.status, resource: body.resource, revision: body.revision };
  }

  async updateFlag(update: { payload: FlagPayload; revision: string }): Promise<FlagUpdateResult> {
    const response = await this.request({
      method: 'PUT',
      url: this.resourcesUrl(),
      body: { key: update.payload.key, payload: update.payload, revision: update.revision },
    });

    if (!response.ok) {
      return { status: response.status };
    }

    const body = (await response.json()) as ResourceResponseBody;

    return { status: response.status, revision: body.revision };
  }

  private resourcesUrl(): string {
    const { baseUrl, environmentKey, namespaceKey } = this.options;

    return `${baseUrl}/api/v2/environments/${environmentKey}/namespaces/${namespaceKey}/resources`;
  }

  private async request(options: { method: string; url: string; body?: unknown }): Promise<Response> {
    return fetch(options.url, {
      method: options.method,
      headers: {
        'Content-Type': 'application/json',
        ...(this.options.token === undefined ? {} : { Authorization: `Bearer ${this.options.token}` }),
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
  }
}
