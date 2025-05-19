import io from 'socket.io-client';
import { TrafficEntry, RequestDetails, ResponseDetails } from '../store';

// 事件类型
export enum WebSocketEvent {
  CONNECT = 'connect',
  DISCONNECT = 'disconnect',
  CONNECT_ERROR = 'connect_error',
  TRAFFIC_ENTRIES = 'traffic_entries',
  TRAFFIC_NEW_ENTRY = 'traffic_new_entry', 
  TRAFFIC_CLEAR = 'traffic_clear',
  REQUEST_DETAILS = 'request_details',
  RESPONSE_DETAILS = 'response_details'
}

// 回调函数类型
export type TrafficEntriesCallback = (entries: TrafficEntry[]) => void;
export type TrafficNewEntryCallback = (entry: TrafficEntry) => void;
export type TrafficClearCallback = () => void;
export type RequestDetailsCallback = (details: RequestDetails) => void;
export type ResponseDetailsCallback = (details: ResponseDetails) => void;
export type ErrorCallback = (error: Error) => void;
export type ConnectCallback = () => void;
export type DisconnectCallback = (reason: string) => void;

// 管理WebSocket连接和事件的服务类
class WebSocketService {
  private socket: any = null; // 使用any类型避免类型错误
  private url: string = '';
  
  // 初始化WebSocket连接
  public init(url: string = ''): void {
    if (this.socket) {
      this.disconnect();
    }

    // 如果没有提供URL，则使用当前域名+/socket.io路径
    this.url = url || `${window.location.protocol}//${window.location.host}`;
    
    console.log('正在连接WebSocket服务:', this.url);
    
    // socket.io v2版本客户端配置
    this.socket = io(this.url, {
      // 尝试polling优先, 这样可以更好地兼容代理和防火墙环境
      transports: ['polling', 'websocket'],
      autoConnect: true,
      reconnection: true,
      reconnectionAttempts: Infinity,
      reconnectionDelay: 1000,
      reconnectionDelayMax: 5000,
      timeout: 20000,
      forceNew: true
    });
    
    // 连接建立后的日志
    this.socket.on('connect', () => {
      console.log('WebSocket连接成功, ID:', this.socket?.id);
    });
    
    // 连接错误处理
    this.socket.on('connect_error', (error: Error) => {
      console.error('WebSocket连接错误:', error.message);
    });
  }

  // 连接WebSocket
  public connect(): void {
    if (this.socket && !this.socket.connected) {
      this.socket.connect();
    } else if (!this.socket) {
      this.init();
    }
  }

  // 断开WebSocket连接
  public disconnect(): void {
    if (this.socket) {
      this.socket.disconnect();
    }
  }

  // 请求所有流量条目
  public requestTrafficEntries(): void {
    if (this.socket) {
      this.socket.emit(WebSocketEvent.TRAFFIC_ENTRIES);
    }
  }

  // 请求请求详情
  public requestRequestDetails(id: string): void {
    if (this.socket) {
      this.socket.emit(WebSocketEvent.REQUEST_DETAILS, id);
    }
  }

  // 请求响应详情
  public requestResponseDetails(id: string): void {
    if (this.socket) {
      this.socket.emit(WebSocketEvent.RESPONSE_DETAILS, id);
    }
  }

  // 请求清空流量条目
  public requestClearTraffic(): void {
    if (this.socket) {
      this.socket.emit(WebSocketEvent.TRAFFIC_CLEAR);
    }
  }

  // 订阅连接事件
  public onConnect(callback: ConnectCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.CONNECT, callback);
    }
  }

  // 订阅断开连接事件
  public onDisconnect(callback: DisconnectCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.DISCONNECT, callback);
    }
  }

  // 订阅连接错误事件
  public onConnectError(callback: ErrorCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.CONNECT_ERROR, callback);
    }
  }

  // 订阅流量条目事件
  public onTrafficEntries(callback: TrafficEntriesCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_ENTRIES, callback);
    }
  }

  // 订阅新流量条目事件
  public onNewTrafficEntry(callback: TrafficNewEntryCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_NEW_ENTRY, callback);
    }
  }

  // 订阅清空流量条目事件
  public onTrafficClear(callback: TrafficClearCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_CLEAR, callback);
    }
  }

  // 订阅请求详情事件
  public onRequestDetails(callback: RequestDetailsCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.REQUEST_DETAILS, callback);
    }
  }

  // 订阅响应详情事件
  public onResponseDetails(callback: ResponseDetailsCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.RESPONSE_DETAILS, callback);
    }
  }

  // 取消订阅所有事件
  public offAll(): void {
    if (this.socket) {
      this.socket.off();
    }
  }

  // 检查连接状态
  public isConnected(): boolean {
    return this.socket ? this.socket.connected : false;
  }
  
  // 获取连接方式
  public getTransport(): string {
    if (!this.socket || !this.socket.connected) {
      return '未连接';
    }
    
    try {
      // 适应socket.io v2格式
      return this.socket.io.engine && this.socket.io.engine.transport
        ? this.socket.io.engine.transport.name
        : '未知';
    } catch (e) {
      return '未知';
    }
  }
}

// 导出单例实例
export const websocketService = new WebSocketService();
export default websocketService; 