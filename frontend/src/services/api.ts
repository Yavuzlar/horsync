import type {
  AuditLog,
  AuthSession,
  Device,
  DeviceEnrollment,
  DeviceEnrollmentInput,
  DeviceRegistrationInput,
  FileEntry,
  InstanceSettings,
  LoginInput,
  PerformancePoint,
  Rule,
  Stats,
  UploadChunkResult,
  UploadSession,
  UploadSessionInput,
  User,
  P2PPeersResponse,
} from '../lib/types';

let authToken = localStorage.getItem('yavuzlar_auth_token') ?? '';

function buildHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  if (authToken) {
    headers.set('Authorization', `Bearer ${authToken}`);
  }
  return headers;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: buildHeaders(init?.headers),
  });

  if (!res.ok) {
    let message = `Request failed with status ${res.status}`;
    try {
      const payload = await res.json();
      if (payload?.error) {
        message = payload.error;
      }
    } catch {
      // Ignore JSON parse failures and keep fallback message.
    }
    throw new Error(message);
  }

  return res.json();
}

export const api = {
  setToken(token: string) {
    authToken = token;
    if (token) {
      localStorage.setItem('yavuzlar_auth_token', token);
    } else {
      localStorage.removeItem('yavuzlar_auth_token');
    }
  },

  getToken() {
    return authToken;
  },

  login(input: LoginInput): Promise<AuthSession> {
    return request<AuthSession>('/api/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  },

  me(): Promise<User> {
    return request<User>('/api/auth/me');
  },

  getStats(): Promise<Stats> {
    return request<Stats>('/api/stats');
  },

  getPerformance(): Promise<PerformancePoint[]> {
    return request<PerformancePoint[]>('/api/performance');
  },

  getNodes() {
    return request('/api/nodes');
  },

  getRules(): Promise<Rule[]> {
    return request<Rule[]>('/api/rules');
  },

  toggleRule(id: number): Promise<Rule> {
    return request<Rule>(`/api/rules/${id}/toggle`, {
      method: 'POST',
    });
  },

  getFiles(): Promise<FileEntry[]> {
    return request<FileEntry[]>('/api/files');
  },

  getSecurityLogs(): Promise<AuditLog[]> {
    return request<AuditLog[]>('/api/security/logs');
  },

  getAuditLogs(): Promise<AuditLog[]> {
    return request<AuditLog[]>('/api/audit/logs');
  },

  getDevices(): Promise<Device[]> {
    return request<Device[]>('/api/devices');
  },

  createEnrollment(input: DeviceEnrollmentInput): Promise<DeviceEnrollment> {
    return request<DeviceEnrollment>('/api/device-enrollments', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  },

  getEnrollments(): Promise<DeviceEnrollment[]> {
    return request<DeviceEnrollment[]>('/api/device-enrollments');
  },

  registerDevice(input: DeviceRegistrationInput): Promise<Device> {
    return request<Device>('/api/devices/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  },

  approveDevice(id: string): Promise<Device> {
    return request<Device>(`/api/devices/${id}/approve`, {
      method: 'POST',
    });
  },

  rejectDevice(id: string): Promise<Device> {
    return request<Device>(`/api/devices/${id}/reject`, {
      method: 'POST',
    });
  },

  getInstanceSettings(): Promise<InstanceSettings> {
    return request<InstanceSettings>('/api/settings/instance');
  },

  updateInstanceSettings(input: InstanceSettings): Promise<InstanceSettings> {
    return request<InstanceSettings>('/api/settings/instance', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  },

  createUploadSession(input: UploadSessionInput, fingerprint?: string): Promise<UploadSession> {
    return request<UploadSession>('/api/uploads/sessions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(fingerprint ? { 'X-Device-Fingerprint': fingerprint } : {}),
      },
      body: JSON.stringify(input),
    });
  },

  getUploadSession(id: string): Promise<UploadSession> {
    return request<UploadSession>(`/api/uploads/${id}`);
  },

  async uploadChunk(id: string, chunkIndex: number, payload: Uint8Array, chunkSha256?: string): Promise<UploadChunkResult> {
    const headers = buildHeaders();
    headers.set('Content-Type', 'application/octet-stream');
    if (chunkSha256) {
      headers.set('X-Chunk-SHA256', chunkSha256);
    }

    const res = await fetch(`/api/uploads/${id}/chunks/${chunkIndex}`, {
      method: 'PUT',
      headers,
      body: payload,
    });

    if (!res.ok) {
      let message = `Request failed with status ${res.status}`;
      try {
        const body = await res.json();
        if (body?.error) {
          message = body.error;
        }
      } catch {
        // Ignore JSON parse failures.
      }
      throw new Error(message);
    }

    return res.json();
  },

  finalizeUpload(id: string): Promise<FileEntry> {
    return request<FileEntry>(`/api/uploads/${id}/finalize`, {
      method: 'POST',
    });
  },

  getP2PPeers(): Promise<P2PPeersResponse> {
    return request<P2PPeersResponse>('/api/p2p/peers');
  },

  wipeMetadata(name: string): Promise<{ status: string; sha256: string }> {
    return request<{ status: string; sha256: string }>(`/api/files/${name}/wipe-metadata`, {
      method: 'POST',
    });
  },
};
