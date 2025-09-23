import io from 'socket.io-client';

import { HttpMessage, TrafficEntry } from '@/types/traffic';

export enum TrafficSocketEvent {
  CONNECT = 'connect',
  DISCONNECT = 'disconnect',
  CONNECT_ERROR = 'connect_error',
  TRAFFIC_ENTRIES = 'traffic_entries',
  TRAFFIC_NEW_ENTRY = 'traffic_new_entry',
  TRAFFIC_CLEAR = 'traffic_clear',
  REQUEST_DETAILS = 'request_details',
  RESPONSE_DETAILS = 'response_details',
}

const DEFAULT_WS_URL = import.meta.env.VITE_PROXYCRAFT_SOCKET_URL ?? 'http://localhost:8081';

type Unsubscribe = () => void;

class TrafficWebSocketService {
  private socket?: ReturnType<typeof io>;
  private url: string = DEFAULT_WS_URL;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private hasRequestedEntries = false;

  init(url: string = DEFAULT_WS_URL) {
    this.url = url;
    if (this.socket) {
      return;
    }

    if (typeof window === 'undefined') {
      return;
    }

    this.socket = io(this.url, {
      transports: ['websocket'],
      reconnection: true,
      reconnectionAttempts: Infinity,
      reconnectionDelay: 1000,
      reconnectionDelayMax: 5000,
      timeout: 20000,
      autoConnect: false,
    });
  }

  private getSocket(): ReturnType<typeof io> | undefined {
    if (!this.socket) {
      this.init(this.url);
    }
    return this.socket;
  }

  connect(url?: string) {
    if (url && url !== this.url) {
      this.dispose();
      this.url = url;
    }
    const socket = this.getSocket();
    if (!socket) {
      return;
    }

    if (!socket.connected) {
      socket.connect();
    }
  }

  disconnect() {
    const socket = this.getSocket();
    socket?.disconnect();
    this.clearReconnectTimer();
    this.hasRequestedEntries = false;
  }

  reconnect() {
    const socket = this.getSocket();
    if (!socket) {
      return;
    }
    this.disconnect();
    this.clearReconnectTimer();
    this.reconnectTimer = setTimeout(() => {
      this.hasRequestedEntries = false;
      this.connect();
      this.reconnectTimer = null;
    }, 1000);
  }

  dispose() {
    this.clearReconnectTimer();
    if (this.socket) {
      this.socket.removeAllListeners();
      this.socket.disconnect();
      this.socket = undefined;
    }
    this.hasRequestedEntries = false;
  }

  requestTrafficEntries(force = false) {
    const socket = this.getSocket();
    if (!socket) {
      return;
    }
    if (this.hasRequestedEntries && !force) {
      return;
    }
    if (socket.connected) {
      socket.emit(TrafficSocketEvent.TRAFFIC_ENTRIES);
      this.hasRequestedEntries = true;
    } else {
      this.connect();
    }
  }

  requestRequestDetails(id: string) {
    const socket = this.getSocket();
    if (socket?.connected) {
      socket.emit(TrafficSocketEvent.REQUEST_DETAILS, id);
    }
  }

  requestResponseDetails(id: string) {
    const socket = this.getSocket();
    if (socket?.connected) {
      socket.emit(TrafficSocketEvent.RESPONSE_DETAILS, id);
    }
  }

  requestClearTraffic() {
    const socket = this.getSocket();
    if (socket?.connected) {
      socket.emit(TrafficSocketEvent.TRAFFIC_CLEAR);
    }
  }

  onConnect(callback: () => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.CONNECT, callback);
    return () => socket?.off(TrafficSocketEvent.CONNECT, callback);
  }

  onDisconnect(callback: (reason: string) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.DISCONNECT, callback);
    return () => socket?.off(TrafficSocketEvent.DISCONNECT, callback);
  }

  onConnectError(callback: (error: Error) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.CONNECT_ERROR, callback);
    return () => socket?.off(TrafficSocketEvent.CONNECT_ERROR, callback);
  }

  onTrafficEntries(callback: (entries: TrafficEntry[]) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.TRAFFIC_ENTRIES, callback);
    return () => socket?.off(TrafficSocketEvent.TRAFFIC_ENTRIES, callback);
  }

  onNewTrafficEntry(callback: (entry: TrafficEntry) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.TRAFFIC_NEW_ENTRY, callback);
    return () => socket?.off(TrafficSocketEvent.TRAFFIC_NEW_ENTRY, callback);
  }

  onTrafficClear(callback: () => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.TRAFFIC_CLEAR, callback);
    return () => socket?.off(TrafficSocketEvent.TRAFFIC_CLEAR, callback);
  }

  onRequestDetails(callback: (details: HttpMessage) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.REQUEST_DETAILS, callback);
    return () => socket?.off(TrafficSocketEvent.REQUEST_DETAILS, callback);
  }

  onResponseDetails(callback: (details: HttpMessage) => void): Unsubscribe {
    const socket = this.getSocket();
    socket?.on(TrafficSocketEvent.RESPONSE_DETAILS, callback);
    return () => socket?.off(TrafficSocketEvent.RESPONSE_DETAILS, callback);
  }

  isConnected(): boolean {
    const socket = this.getSocket();
    return Boolean(socket?.connected);
  }

  getTransport(): string {
    const socket = this.getSocket();
    if (!socket || !socket.connected) {
      return 'disconnected';
    }
    return socket.io.engine.transport.name || 'unknown';
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

export const trafficSocket = new TrafficWebSocketService();
