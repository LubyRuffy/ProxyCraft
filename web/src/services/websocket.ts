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

/**
 * WebSocket服务类 - 提供与后端的实时通信
 */
class WebSocketService {
  private socket: any = null;
  private url: string = '';
  private reconnectTimer: number | null = null;
  
  /**
   * 初始化WebSocket连接
   * @param url 服务器URL，默认为http://localhost:8081
   */
  public init(url: string = 'http://localhost:8081'): void {
    // 如果已经有socket实例，先清理
    this.cleanup();
    
    // 保存URL
    this.url = url;
    
    console.log('初始化WebSocket连接...');
    
    // 创建socket实例，采用最简单稳定的配置
    this.socket = io(this.url, {
      transports: ['websocket'],
      reconnection: true,        // 启用自动重连
      reconnectionAttempts: Infinity, // 无限重试
      reconnectionDelay: 1000,   // 初始重连延迟1秒
      reconnectionDelayMax: 5000, // 最大重连延迟5秒
      timeout: 20000,            // 连接超时20秒
      autoConnect: true          // 自动连接
    });
    
    this.attachEventHandlers();
    
    // 测试连接，如果5秒内没有连接成功，尝试强制重连
    setTimeout(() => {
      if (this.socket && !this.socket.connected) {
        console.log('WebSocket连接超时，尝试重新连接');
        this.reconnect();
      }
    }, 5000);
  }
  
  /**
   * 连接WebSocket
   */
  public connect(): void {
    if (!this.socket) {
      this.init();
      return;
    }
    
    if (!this.socket.connected) {
      console.log('尝试连接WebSocket...');
      this.socket.connect();
    }
  }
  
  /**
   * 断开WebSocket连接
   */
  public disconnect(): void {
    if (this.socket) {
      console.log('断开WebSocket连接');
      this.socket.disconnect();
    }
    
    this.cleanup();
  }
  
  /**
   * 重新连接WebSocket
   */
  public reconnect(): void {
    console.log('重新连接WebSocket');
    this.disconnect();
    
    // 防止短时间内多次重连
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    
    this.reconnectTimer = window.setTimeout(() => {
      this.init(this.url);
      this.reconnectTimer = null;
    }, 1000);
  }
  
  /**
   * 清理资源
   */
  private cleanup(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    
    if (this.socket) {
      try {
        this.socket.offAny(); // 移除所有事件监听
        this.socket.disconnect();
      } catch (e) {
        console.error('清理socket时出错:', e);
      }
      this.socket = null;
    }
  }
  
  /**
   * 添加事件处理器
   */
  private attachEventHandlers(): void {
    if (!this.socket) return;
    
    // 连接成功事件
    this.socket.on(WebSocketEvent.CONNECT, () => {
      console.log('WebSocket连接成功, ID:', this.socket?.id, '传输方式:', this.getTransport());
    });
    
    // 连接错误事件
    this.socket.on(WebSocketEvent.CONNECT_ERROR, (error: Error) => {
      console.error('WebSocket连接错误:', error.message);
    });
    
    // 断开连接事件
    this.socket.on(WebSocketEvent.DISCONNECT, (reason: string) => {
      console.log(`WebSocket断开连接, 原因:`, reason);
    });
    
    // 监听重连尝试
    this.socket.io.on('reconnect_attempt', (attempt: number) => {
      console.log(`WebSocket尝试重连 (${attempt})`);
    });
    
    // 重连成功
    this.socket.io.on('reconnect', (attempt: number) => {
      console.log(`WebSocket重连成功，经过${attempt}次尝试`);
    });
  }
  
  /**
   * 请求所有流量条目
   */
  public requestTrafficEntries(): void {
    if (this.socket && this.socket.connected) {
      console.log('请求所有流量条目');
      this.socket.emit(WebSocketEvent.TRAFFIC_ENTRIES);
    } else {
      console.warn('WebSocket未连接，首先尝试连接...');
      this.connect();
      // 连接成功后会自动请求流量条目
      this.onConnect(() => {
        this.requestTrafficEntries();
      });
    }
  }
  
  /**
   * 请求请求详情
   */
  public requestRequestDetails(id: string): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.REQUEST_DETAILS, id);
    } else {
      console.warn('WebSocket未连接，无法获取请求详情');
    }
  }
  
  /**
   * 请求响应详情
   */
  public requestResponseDetails(id: string): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.RESPONSE_DETAILS, id);
    } else {
      console.warn('WebSocket未连接，无法获取响应详情');
    }
  }
  
  /**
   * 请求清空流量条目
   */
  public requestClearTraffic(): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.TRAFFIC_CLEAR);
    } else {
      console.warn('WebSocket未连接，无法清空流量');
    }
  }
  
  /**
   * 订阅连接事件
   */
  public onConnect(callback: ConnectCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.CONNECT, callback);
    }
  }
  
  /**
   * 订阅断开连接事件
   */
  public onDisconnect(callback: DisconnectCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.DISCONNECT, callback);
    }
  }
  
  /**
   * 订阅连接错误事件
   */
  public onConnectError(callback: ErrorCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.CONNECT_ERROR, callback);
    }
  }
  
  /**
   * 订阅流量条目事件
   */
  public onTrafficEntries(callback: TrafficEntriesCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_ENTRIES, callback);
    }
  }
  
  /**
   * 订阅新流量条目事件
   */
  public onNewTrafficEntry(callback: TrafficNewEntryCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_NEW_ENTRY, callback);
    }
  }
  
  /**
   * 订阅清空流量条目事件
   */
  public onTrafficClear(callback: TrafficClearCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.TRAFFIC_CLEAR, callback);
    }
  }
  
  /**
   * 订阅请求详情事件
   */
  public onRequestDetails(callback: RequestDetailsCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.REQUEST_DETAILS, callback);
    }
  }
  
  /**
   * 订阅响应详情事件
   */
  public onResponseDetails(callback: ResponseDetailsCallback): void {
    if (this.socket) {
      this.socket.on(WebSocketEvent.RESPONSE_DETAILS, callback);
    }
  }
  
  /**
   * 取消订阅所有事件
   */
  public offAll(): void {
    if (this.socket) {
      this.socket.offAny();
    }
  }
  
  /**
   * 检查连接状态
   */
  public isConnected(): boolean {
    return this.socket ? this.socket.connected : false;
  }
  
  /**
   * 获取连接方式
   */
  public getTransport(): string {
    if (!this.socket || !this.socket.connected) {
      return '未连接';
    }
    
    try {
      return this.socket.io.engine && this.socket.io.engine.transport
        ? this.socket.io.engine.transport.name
        : '未知';
    } catch (e) {
      return '未知';
    }
  }
  
  /**
   * 获取连接状态信息
   */
  public getConnectionInfo(): object {
    return {
      connected: this.isConnected(),
      transport: this.getTransport(),
      url: this.url,
      socketId: this.socket?.id || '无'
    };
  }
}

// 导出单例实例
export const websocketService = new WebSocketService();
export default websocketService; 