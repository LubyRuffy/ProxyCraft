import { createStore } from 'vuex';
import axios from 'axios';
import websocketService, { WebSocketEvent } from '../services/websocket';

export interface TrafficEntry {
  id: string;
  startTime: string;
  endTime: string;
  duration: number;
  host: string;
  method: string;
  url: string;
  path: string;
  statusCode: number;
  contentType: string;
  contentSize: number;
  isSSE: boolean;
  isHTTPS: boolean;
  error?: string;
}

export interface RequestDetails {
  headers: Record<string, string>;
  body: any;
}

export interface ResponseDetails {
  headers: Record<string, string>;
  body: any;
}

export interface State {
  trafficEntries: TrafficEntry[];
  selectedEntry: TrafficEntry | null;
  requestDetails: RequestDetails | null;
  responseDetails: ResponseDetails | null;
  loading: boolean;
  error: string | null;
  connected: boolean;
}

export default createStore<State>({
  state: {
    trafficEntries: [],
    selectedEntry: null,
    requestDetails: null,
    responseDetails: null,
    loading: false,
    error: null,
    connected: false,
  },
  mutations: {
    setTrafficEntries(state, entries: TrafficEntry[]) {
      state.trafficEntries = entries;
    },
    addTrafficEntry(state, entry: TrafficEntry) {
      // 避免重复添加
      if (!state.trafficEntries.find(e => e.id === entry.id)) {
        state.trafficEntries.unshift(entry); // 添加到开头，最新的在上面
      }
    },
    setSelectedEntry(state, entry: TrafficEntry | null) {
      state.selectedEntry = entry;
    },
    setRequestDetails(state, details: RequestDetails | null) {
      state.requestDetails = details;
    },
    setResponseDetails(state, details: ResponseDetails | null) {
      state.responseDetails = details;
    },
    setLoading(state, loading: boolean) {
      state.loading = loading;
    },
    setError(state, error: string | null) {
      state.error = error;
    },
    setConnected(state, connected: boolean) {
      state.connected = connected;
    },
    clearTrafficEntries(state) {
      state.trafficEntries = [];
      state.selectedEntry = null;
      state.requestDetails = null;
      state.responseDetails = null;
    }
  },
  actions: {
    // 初始化WebSocket连接
    initWebSocket({ commit, dispatch }) {
      websocketService.init();
      
      // 连接成功时的处理
      websocketService.onConnect(() => {
        commit('setConnected', true);
        commit('setError', null);
        dispatch('fetchTrafficEntries');
      });
      
      // 断开连接时的处理
      websocketService.onDisconnect((reason) => {
        commit('setConnected', false);
        commit('setError', `WebSocket连接已断开: ${reason}`);
      });
      
      // 连接出错时的处理
      websocketService.onConnectError((error) => {
        commit('setConnected', false);
        commit('setError', `WebSocket连接错误: ${error.message}`);
      });
      
      // 接收所有流量条目的处理
      websocketService.onTrafficEntries((entries) => {
        commit('setTrafficEntries', entries || []);
        commit('setLoading', false);
      });
      
      // 接收新的流量条目的处理
      websocketService.onNewTrafficEntry((entry) => {
        commit('addTrafficEntry', entry);
      });
      
      // 接收请求详情的处理
      websocketService.onRequestDetails((details) => {
        commit('setRequestDetails', details);
        commit('setLoading', false);
      });
      
      // 接收响应详情的处理
      websocketService.onResponseDetails((details) => {
        commit('setResponseDetails', details);
        commit('setLoading', false);
      });
      
      // 接收清空流量条目的处理
      websocketService.onTrafficClear(() => {
        commit('clearTrafficEntries');
        commit('setLoading', false);
      });
      
      // 连接WebSocket
      websocketService.connect();
    },
    
    // 获取所有流量条目
    fetchTrafficEntries({ commit }) {
      commit('setLoading', true);
      commit('setError', null);
      
      if (websocketService.isConnected()) {
        websocketService.requestTrafficEntries();
      } else {
        // 回退到HTTP请求
        axios.get('/api/traffic')
          .then(response => {
            commit('setTrafficEntries', response.data.entries || []);
          })
          .catch(error => {
            commit('setError', '获取流量数据失败');
            console.error('Failed to fetch traffic entries:', error);
          })
          .finally(() => {
            commit('setLoading', false);
          });
      }
    },
    
    // 选择一个流量条目
    async selectEntry({ commit, dispatch }, entry: TrafficEntry | null) {
      commit('setSelectedEntry', entry);
      if (entry) {
        await Promise.all([
          dispatch('fetchRequestDetails', entry.id),
          dispatch('fetchResponseDetails', entry.id),
        ]);
      } else {
        commit('setRequestDetails', null);
        commit('setResponseDetails', null);
      }
    },
    
    // 获取请求详情
    fetchRequestDetails({ commit }, id: string) {
      commit('setLoading', true);
      
      if (websocketService.isConnected()) {
        websocketService.requestRequestDetails(id);
      } else {
        // 回退到HTTP请求
        axios.get(`/api/traffic/${id}/request`)
          .then(response => {
            commit('setRequestDetails', response.data);
          })
          .catch(error => {
            console.error('Failed to fetch request details:', error);
            commit('setRequestDetails', null);
          })
          .finally(() => {
            commit('setLoading', false);
          });
      }
    },
    
    // 获取响应详情
    fetchResponseDetails({ commit }, id: string) {
      commit('setLoading', true);
      
      if (websocketService.isConnected()) {
        websocketService.requestResponseDetails(id);
      } else {
        // 回退到HTTP请求
        axios.get(`/api/traffic/${id}/response`)
          .then(response => {
            commit('setResponseDetails', response.data);
          })
          .catch(error => {
            console.error('Failed to fetch response details:', error);
            commit('setResponseDetails', null);
          })
          .finally(() => {
            commit('setLoading', false);
          });
      }
    },
    
    // 清空所有流量条目
    clearTrafficEntries({ commit }) {
      commit('setLoading', true);
      
      if (websocketService.isConnected()) {
        websocketService.requestClearTraffic();
      } else {
        // 回退到HTTP请求
        axios.delete('/api/traffic')
          .then(() => {
            commit('clearTrafficEntries');
          })
          .catch(error => {
            commit('setError', '清空流量数据失败');
            console.error('Failed to clear traffic entries:', error);
          })
          .finally(() => {
            commit('setLoading', false);
          });
      }
    },
  },
  getters: {
    // 获取所有流量条目
    allTrafficEntries: (state) => state.trafficEntries,
    
    // 获取选中的流量条目
    selectedTrafficEntry: (state) => state.selectedEntry,
    
    // 获取请求详情
    requestDetails: (state) => state.requestDetails,
    
    // 获取响应详情
    responseDetails: (state) => state.responseDetails,
    
    // 是否正在加载
    isLoading: (state) => state.loading,
    
    // 获取错误信息
    error: (state) => state.error,
    
    // 获取连接状态
    isConnected: (state) => state.connected,
  },
}); 