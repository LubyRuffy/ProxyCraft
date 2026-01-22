export type TrafficEntry = {
  id: string;
  startTime?: string;
  endTime?: string;
  duration: number;
  host: string;
  tags?: string[];
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
  isTimeout: boolean;
  error?: string;
};

export type LLMRequestInfo = {
  prompt?: string;
  toolCalls?: unknown;
  tools?: unknown;
};

export type LLMResponseInfo = {
  content?: string;
  toolCalls?: unknown;
  reasoning?: string;
};

export type LLMExtracted = {
  provider?: string;
  model?: string;
  streaming?: boolean;
  request?: LLMRequestInfo;
  response?: LLMResponseInfo;
};

export type HttpMessage = {
  headers: Record<string, string>;
  body?: unknown;
  llm?: LLMExtracted;
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
