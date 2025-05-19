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
  private connecting: boolean = false;
  private connectionAttempts: number = 0;
  private maxReconnectAttempts: number = 5;
  
  // 初始化WebSocket连接
  public init(url: string = ''): void {
    if (this.socket) {
      this.disconnect();
    }

    // 如果已经在尝试连接，则跳过
    if (this.connecting) {
      console.log('已经在尝试连接，跳过重复连接请求');
      return;
    }

    this.connecting = true;
    this.connectionAttempts++;

    // 如果没有提供URL，则使用当前域名+/socket.io路径
    this.url = url || `${window.location.protocol}//${window.location.host}`;
    
    console.log(`正在连接WebSocket服务(尝试 ${this.connectionAttempts}/${this.maxReconnectAttempts}):`, this.url);
    
    // 直接使用WebSocket传输方式，避免使用polling
    const transport = ['websocket'];

    // socket.io客户端配置
    this.socket = io(this.url, {
      transports: transport,
      autoConnect: true,
      reconnection: true,
      reconnectionAttempts: 3,    // 增加自动重连次数
      reconnectionDelay: 1000,    // 重连延迟1秒
      reconnectionDelayMax: 3000, // 最大重连延迟3秒
      timeout: 20000,             // 增加超时时间
      forceNew: true,
      query: { timestamp: new Date().getTime() } // 添加时间戳避免缓存问题
    });
    
    this.setupEventHandlers();
  }
  
  // 设置事件处理器
  private setupEventHandlers(): void {
    if (!this.socket) return;
    
    // 连接成功事件
    this.socket.on('connect', () => {
      console.log('WebSocket连接成功, ID:', this.socket?.id, '传输方式:', this.getTransport());
      this.connecting = false;
      this.connectionAttempts = 0; // 重置连接尝试计数
    });
    
    // 连接错误事件
    this.socket.on('connect_error', (error: Error) => {
      console.error('WebSocket连接错误:', error.message, '传输方式:', this.getTransport());
      this.connecting = false;
      
      // 在尝试次数达到上限前尝试重连
      if (this.connectionAttempts < this.maxReconnectAttempts) {
        // 使用指数退避策略，每次失败后等待时间增加
        const delay = Math.min(1000 * Math.pow(1.5, this.connectionAttempts-1), 10000);
        console.log(`将在 ${delay/1000} 秒后尝试重新连接...`);
        
        setTimeout(() => {
          // 始终使用WebSocket，不切换到polling
          this.init(this.url);
        }, delay);
      } else {
        console.error(`已达到最大重连尝试次数(${this.maxReconnectAttempts})，将在30秒后自动重试`);
        // 30秒后自动重置连接状态并重试
        setTimeout(() => {
          console.log('自动重置连接状态并重试');
          this.connectionAttempts = 0;
          this.init(this.url);
        }, 30000);
      }
    });
    
    // 断开连接事件
    this.socket.on('disconnect', (reason: string) => {
      console.log(`WebSocket断开连接, 原因: ${reason}`);
      this.connecting = false;
      
      // 如果是服务器主动断开或ping超时，尝试重新连接
      if (reason === 'io server disconnect' || reason === 'ping timeout') {
        console.log('服务器断开连接，尝试重新连接...');
        this.connect();
      }
    });
    
    // 重连尝试日志
    this.socket.io.on('reconnect_attempt', (attempt: number) => {
      console.log(`WebSocket尝试重连 (${attempt}), 传输方式: ${this.getTransport()}`);
    });
    
    // 重连成功日志
    this.socket.io.on('reconnect', (attempt: number) => {
      console.log(`WebSocket重连成功，经过${attempt}次尝试`);
    });
    
    // 重连错误
    this.socket.io.on('reconnect_error', (error: Error) => {
      console.error('WebSocket重连错误:', error.message);
    });
    
    // 重连失败
    this.socket.io.on('reconnect_failed', () => {
      console.error('WebSocket重连失败，已达到最大尝试次数');
    });
    
    // ping事件
    this.socket.io.engine?.on('ping', () => {
      console.log('发送ping到服务器');
    });
    
    // pong事件
    this.socket.io.engine?.on('pong', (latency: number) => {
      console.log(`收到服务器pong响应，延迟: ${latency}ms`);
    });
  }

  // 连接WebSocket
  public connect(): void {
    if (this.connecting) {
      console.log('已经在尝试连接中，请稍候...');
      return;
    }
    
    if (this.socket && !this.socket.connected) {
      console.log('尝试重新连接WebSocket...');
      this.socket.connect();
    } else if (!this.socket) {
      console.log('创建新的WebSocket连接...');
      this.init();
    } else {
      console.log('WebSocket已连接');
    }
  }

  // 断开WebSocket连接
  public disconnect(): void {
    if (this.socket) {
      console.log('断开WebSocket连接');
      this.socket.disconnect();
      this.socket = null;
    }
    this.connecting = false;
  }

  // 重置并重新连接
  public reconnect(): void {
    console.log('重置连接状态并重新连接');
    this.disconnect();
    this.connectionAttempts = 0;
    this.connect();
  }

  // 请求所有流量条目
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

  // 请求请求详情
  public requestRequestDetails(id: string): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.REQUEST_DETAILS, id);
    } else {
      console.warn('WebSocket未连接，无法获取请求详情');
    }
  }

  // 请求响应详情
  public requestResponseDetails(id: string): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.RESPONSE_DETAILS, id);
    } else {
      console.warn('WebSocket未连接，无法获取响应详情');
    }
  }

  // 请求清空流量条目
  public requestClearTraffic(): void {
    if (this.socket && this.socket.connected) {
      this.socket.emit(WebSocketEvent.TRAFFIC_CLEAR);
    } else {
      console.warn('WebSocket未连接，无法清空流量');
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
      // 适应socket.io格式
      return this.socket.io.engine && this.socket.io.engine.transport
        ? this.socket.io.engine.transport.name
        : '未知';
    } catch (e) {
      return '未知';
    }
  }
  
  // 获取连接状态信息
  public getConnectionInfo(): object {
    return {
      connected: this.isConnected(),
      transport: this.getTransport(),
      attempts: this.connectionAttempts,
      maxAttempts: this.maxReconnectAttempts,
      connecting: this.connecting,
      url: this.url,
      socketId: this.socket?.id || '无'
    };
  }
}

// 导出单例实例
export const websocketService = new WebSocketService();
export default websocketService; 