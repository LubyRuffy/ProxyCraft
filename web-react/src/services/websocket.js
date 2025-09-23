import io from 'socket.io-client';
export var TrafficSocketEvent;
(function (TrafficSocketEvent) {
    TrafficSocketEvent["CONNECT"] = "connect";
    TrafficSocketEvent["DISCONNECT"] = "disconnect";
    TrafficSocketEvent["CONNECT_ERROR"] = "connect_error";
    TrafficSocketEvent["TRAFFIC_ENTRIES"] = "traffic_entries";
    TrafficSocketEvent["TRAFFIC_NEW_ENTRY"] = "traffic_new_entry";
    TrafficSocketEvent["TRAFFIC_CLEAR"] = "traffic_clear";
    TrafficSocketEvent["REQUEST_DETAILS"] = "request_details";
    TrafficSocketEvent["RESPONSE_DETAILS"] = "response_details";
})(TrafficSocketEvent || (TrafficSocketEvent = {}));
const DEFAULT_WS_URL = import.meta.env.VITE_PROXYCRAFT_SOCKET_URL ?? 'http://localhost:8081';
class TrafficWebSocketService {
    constructor() {
        Object.defineProperty(this, "socket", {
            enumerable: true,
            configurable: true,
            writable: true,
            value: void 0
        });
        Object.defineProperty(this, "url", {
            enumerable: true,
            configurable: true,
            writable: true,
            value: DEFAULT_WS_URL
        });
        Object.defineProperty(this, "reconnectTimer", {
            enumerable: true,
            configurable: true,
            writable: true,
            value: null
        });
        Object.defineProperty(this, "hasRequestedEntries", {
            enumerable: true,
            configurable: true,
            writable: true,
            value: false
        });
    }
    init(url = DEFAULT_WS_URL) {
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
    getSocket() {
        if (!this.socket) {
            this.init(this.url);
        }
        return this.socket;
    }
    connect(url) {
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
        }
        else {
            this.connect();
        }
    }
    requestRequestDetails(id) {
        const socket = this.getSocket();
        if (socket?.connected) {
            socket.emit(TrafficSocketEvent.REQUEST_DETAILS, id);
        }
    }
    requestResponseDetails(id) {
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
    onConnect(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.CONNECT, callback);
        return () => socket?.off(TrafficSocketEvent.CONNECT, callback);
    }
    onDisconnect(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.DISCONNECT, callback);
        return () => socket?.off(TrafficSocketEvent.DISCONNECT, callback);
    }
    onConnectError(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.CONNECT_ERROR, callback);
        return () => socket?.off(TrafficSocketEvent.CONNECT_ERROR, callback);
    }
    onTrafficEntries(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.TRAFFIC_ENTRIES, callback);
        return () => socket?.off(TrafficSocketEvent.TRAFFIC_ENTRIES, callback);
    }
    onNewTrafficEntry(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.TRAFFIC_NEW_ENTRY, callback);
        return () => socket?.off(TrafficSocketEvent.TRAFFIC_NEW_ENTRY, callback);
    }
    onTrafficClear(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.TRAFFIC_CLEAR, callback);
        return () => socket?.off(TrafficSocketEvent.TRAFFIC_CLEAR, callback);
    }
    onRequestDetails(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.REQUEST_DETAILS, callback);
        return () => socket?.off(TrafficSocketEvent.REQUEST_DETAILS, callback);
    }
    onResponseDetails(callback) {
        const socket = this.getSocket();
        socket?.on(TrafficSocketEvent.RESPONSE_DETAILS, callback);
        return () => socket?.off(TrafficSocketEvent.RESPONSE_DETAILS, callback);
    }
    isConnected() {
        const socket = this.getSocket();
        return Boolean(socket?.connected);
    }
    getTransport() {
        const socket = this.getSocket();
        if (!socket || !socket.connected) {
            return 'disconnected';
        }
        return socket.io.engine.transport.name || 'unknown';
    }
    clearReconnectTimer() {
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
    }
}
export const trafficSocket = new TrafficWebSocketService();
