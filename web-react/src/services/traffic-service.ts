import { HttpMessage, TrafficDetail, TrafficEntry } from '@/types/traffic';

const API_BASE = '/api';

const MOCK_ENTRIES: TrafficEntry[] = [
  {
    id: 'a1',
    startTime: new Date(Date.now() - 1000).toISOString(),
    endTime: new Date().toISOString(),
    duration: 86,
    host: 'api.example.com',
    method: 'GET',
    url: 'https://api.example.com/v1/profile',
    path: '/v1/profile',
    statusCode: 200,
    contentType: 'application/json',
    contentSize: 20480,
    isSSE: false,
    isSSECompleted: false,
    isHTTPS: true,
    isTimeout: false,
  },
  {
    id: 'a0',
    startTime: new Date(Date.now() - 2000).toISOString(),
    endTime: new Date(Date.now() - 1500).toISOString(),
    duration: 142,
    host: 'auth.example.com',
    method: 'POST',
    url: 'https://auth.example.com/oauth/token',
    path: '/oauth/token',
    statusCode: 201,
    contentType: 'application/json',
    contentSize: 4096,
    isSSE: false,
    isSSECompleted: false,
    isHTTPS: true,
    isTimeout: false,
  },
  {
    id: '9f',
    startTime: new Date(Date.now() - 3000).toISOString(),
    endTime: new Date(Date.now() - 2500).toISOString(),
    duration: 0,
    host: 'stream.example.com',
    method: 'GET',
    url: 'https://stream.example.com/events',
    path: '/events',
    statusCode: 200,
    contentType: 'text/event-stream',
    contentSize: 5120,
    isSSE: true,
    isSSECompleted: false,
    isHTTPS: true,
    isTimeout: false,
  },
];

const MOCK_DETAILS: Record<string, TrafficDetail> = {
  a1: {
    request: {
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer ...',
      },
    },
    response: {
      headers: {
        'Content-Type': 'application/json',
      },
      body: { name: 'Jane' },
    },
  },
};

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'same-origin',
    ...init,
  });

  if (!res.ok) {
    throw new Error(`Request failed with status ${res.status}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return (await res.json()) as T;
}

export async function fetchTrafficEntries(): Promise<TrafficEntry[]> {
  try {
    const data = await fetchJSON<{ entries?: TrafficEntry[] }>('/traffic');
    return data.entries ?? [];
  } catch (error) {
    console.warn('fetchTrafficEntries fallback to mock data:', error);
    return MOCK_ENTRIES;
  }
}

export async function fetchRequestDetail(id: string): Promise<HttpMessage | undefined> {
  try {
    return await fetchJSON<HttpMessage>(`/traffic/${id}/request`);
  } catch (error) {
    console.warn(`fetchRequestDetail fallback for ${id}:`, error);
    return MOCK_DETAILS[id]?.request;
  }
}

export async function fetchResponseDetail(id: string): Promise<HttpMessage | undefined> {
  try {
    return await fetchJSON<HttpMessage>(`/traffic/${id}/response`);
  } catch (error) {
    console.warn(`fetchResponseDetail fallback for ${id}:`, error);
    return MOCK_DETAILS[id]?.response;
  }
}

export async function fetchTrafficDetail(id: string): Promise<TrafficDetail | undefined> {
  const [request, response] = await Promise.all([
    fetchRequestDetail(id),
    fetchResponseDetail(id),
  ]);

  if (!request && !response) {
    return undefined;
  }

  return { request, response };
}

export async function clearTrafficRemote(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/traffic`, {
      method: 'DELETE',
      credentials: 'same-origin',
    });
    if (!res.ok) {
      throw new Error(`Request failed with status ${res.status}`);
    }
    return true;
  } catch (error) {
    console.warn('clearTrafficRemote fallback:', error);
    return false;
  }
}
