export type TrafficEntry = {
  id: string;
  startTime?: string;
  endTime?: string;
  duration: number;
  host: string;
  processName?: string;
  processIcon?: string;
  method: string;
  url: string;
  path: string;
  statusCode: number;
  contentType: string;
  contentSize: number;
  isSSE: boolean;
  isSSECompleted: boolean;
  isHTTPS: boolean;
  error?: string;
};

export type HttpMessage = {
  headers: Record<string, string>;
  body?: unknown;
};

export type TrafficDetail = {
  request?: HttpMessage;
  response?: HttpMessage;
};

export type ConnectionState = {
  connected: boolean;
  transport: string;
  error: string | null;
};
