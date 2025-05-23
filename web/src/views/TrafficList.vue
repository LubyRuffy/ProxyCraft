<template>
  <div class="traffic-list-container">
    <!-- 工具栏 -->
    <div class="toolbar">
      <el-button type="primary" @click="refreshData" :loading="loading" size="small">
        刷新
      </el-button>
      <el-button type="danger" @click="confirmClear" :loading="loading" size="small">
        清空
      </el-button>
      <el-button type="warning" @click="reconnectWebSocket" :loading="reconnecting" size="small">
        重新连接
      </el-button>
      <div class="connection-status">
        <el-tag v-if="isConnected" type="success" size="small" effect="plain">
          WebSocket已连接 ({{ transportMode }})
        </el-tag>
        <el-tag v-else type="danger" size="small" effect="plain">WebSocket已断开</el-tag>
      </div>
      <div class="error-message" v-if="error">
        <el-alert :title="error" type="error" :closable="false" size="small" />
      </div>
    </div>

    <!-- 主界面 -->
    <div class="main-content" :style="{ height: `calc(100vh - 38px - ${detailPanelHeight}px)` }">
      <!-- 请求列表 -->
      <el-table
        :data="trafficEntries"
        stripe
        style="width: 100%"
        height="100%"
        @row-click="handleRowClick"
        v-loading="loading && !detailLoading"
        highlight-current-row
        size="small"
        class="burp-style-table"
        :default-sort="{ prop: 'id', order: 'descending' }"
      >
        <el-table-column prop="method" label="方法" width="60">
          <template #default="scope">
            <span class="method-cell">{{ scope.row.method }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="host_with_schema" label="Host" width="200" show-overflow-tooltip />
        <el-table-column prop="path" label="Path" min-width="180" show-overflow-tooltip>
          <template #default="scope">
            <span class="path-cell">{{ scope.row.path }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="statusCode" label="Code" width="70">
          <template #default="scope">
            <span :class="getStatusClass(scope.row.statusCode)">{{ scope.row.statusCode }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="contentType" label="MIME" width="120" show-overflow-tooltip />
        <el-table-column prop="contentSize" label="Size" width="100">
          <template #default="scope">
            {{ formatBytes(scope.row.contentSize) }}
          </template>
        </el-table-column>
        <el-table-column prop="duration" label="Cost" width="70">
          <template #default="scope">
            {{ scope.row.duration }}ms
          </template>
        </el-table-column>
        <el-table-column width="60">
          <template #default="scope">
            <el-tag v-if="scope.row.isHTTPS" type="success" size="small" effect="plain" round>HTTPS</el-tag>
            <el-tag v-if="scope.row.isSSE" type="warning" size="small" effect="plain" round>SSE</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 可拖动分隔条 -->
    <div
      class="resizer"
      @mousedown="startResize"
      :class="{ 'resizing': isResizing }"
      v-if="selectedEntry"
    ></div>

    <!-- 详情面板 -->
    <div
      class="detail-panel"
      v-if="selectedEntry"
      :style="{ height: `${detailPanelHeight}px` }"
    >
      <request-response-panel
        :request="requestDetails"
        :response="responseDetails"
        :loading="detailLoading"
        :selectedEntry="selectedEntry"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, onUnmounted, watch } from 'vue';
import { useStore } from 'vuex';
import { ElMessageBox } from 'element-plus';
import RequestResponsePanel from '../components/RequestResponsePanel.vue';
import { TrafficEntry } from '../store';
import websocketService, { WebSocketEvent } from '../services/websocket';
import { getStatusClass, formatBytes } from '../utils/formatters';

const store = useStore();
const transportMode = ref('');
const detailLoading = ref(false);
const lastClickedId = ref('');
const clickDebounceTimer = ref<number | null>(null);
const detailPanelHeight = ref(300); // 初始高度
const isResizing = ref(false);
const minHeight = 100; // 最小高度
const maxHeight = 600; // 最大高度
const reconnecting = ref(false); // 是否正在重连
const connectionCheckTimer = ref<number | null>(null); // 连接检查定时器

// 从 store 中获取状态
const trafficEntries = computed(() => store.getters.allTrafficEntries);
const selectedEntry = computed(() => store.getters.selectedTrafficEntry);
const requestDetails = computed(() => store.getters.requestDetails);
const responseDetails = computed(() => store.getters.responseDetails);
const loading = computed(() => store.getters.isLoading);
const error = computed(() => store.getters.error);
const isConnected = computed(() => store.getters.isConnected);

// 拖动调整大小相关方法
const startResize = (e: MouseEvent) => {
  isResizing.value = true;
  document.addEventListener('mousemove', handleMouseMove);
  document.addEventListener('mouseup', stopResize);
  // 阻止默认行为和冒泡
  e.preventDefault();
  e.stopPropagation();
};

const handleMouseMove = (e: MouseEvent) => {
  if (!isResizing.value) return;

  // 计算窗口总高度
  const windowHeight = window.innerHeight;
  // 计算鼠标位置相对于窗口底部的距离
  const fromBottom = windowHeight - e.clientY;

  // 确保在最小和最大高度范围内
  let newHeight = Math.max(minHeight, Math.min(maxHeight, fromBottom));

  // 更新详情面板高度
  detailPanelHeight.value = newHeight;
};

const stopResize = () => {
  isResizing.value = false;
  document.removeEventListener('mousemove', handleMouseMove);
  document.removeEventListener('mouseup', stopResize);
};

// 移除事件监听器，防止内存泄漏
onUnmounted(() => {
  document.removeEventListener('mousemove', handleMouseMove);
  document.removeEventListener('mouseup', stopResize);
});

// 重新连接WebSocket
const reconnectWebSocket = () => {
  reconnecting.value = true;

  // 先断开当前连接
  store.commit('setConnected', false);
  websocketService.disconnect();

  // 延迟一秒后重新连接，确保前一个连接完全关闭
  setTimeout(() => {
    console.log('尝试重新建立WebSocket连接...');
    websocketService.reconnect();

    // 设置5秒超时，如果还没有连接成功就显示错误
    setTimeout(() => {
      if (!websocketService.isConnected()) {
        store.commit('setError', '重连失败，请刷新页面再试');
      }
      reconnecting.value = false;
    }, 5000);
  }, 1000);
};

// 初始化WebSocket并加载数据
onMounted(() => {
  // 初始化WebSocket连接
  console.log('初始化WebSocket连接...');
  store.dispatch('initWebSocket');

  // 初始加载数据
  refreshData();

  // 设置WebSocket响应详情处理器
  setupWebSocketDetailHandlers();

  // 设置定时器检查连接状态并更新传输模式
  connectionCheckTimer.value = window.setInterval(() => {
    transportMode.value = websocketService.getTransport();

    // 如果连接状态与store中存储的状态不一致，更新状态
    const currentConnected = websocketService.isConnected();
    if (currentConnected !== isConnected.value) {
      store.commit('setConnected', currentConnected);

      // 如果连接已恢复，尝试重新获取数据
      if (currentConnected && !isConnected.value) {
        refreshData();
      }
    }
  }, 2000) as unknown as number;
});

// SSE刷新定时器
let sseRefreshTimer: number | null = null;

// 开始SSE详情刷新
const startSSERefresh = (entryId: string) => {
  // 如果已经有定时器在运行，先清除
  if (sseRefreshTimer !== null) {
    clearInterval(sseRefreshTimer);
    sseRefreshTimer = null;
  }

  console.log(`开始SSE刷新定时器，ID: ${entryId}`);

  // 设置新的定时器，每秒获取一次最新的SSE响应内容
  sseRefreshTimer = setInterval(() => {
    if (selectedEntry.value && selectedEntry.value.id === entryId && selectedEntry.value.isSSE) {
      // 检查SSE是否已经完成
      console.log(`检查SSE状态，ID: ${entryId}, isSSECompleted: ${selectedEntry.value.isSSECompleted}`);

      if (selectedEntry.value.isSSECompleted) {
        // 如果SSE已经完成，停止刷新
        console.log(`SSE连接已经结束，停止刷新，ID: ${entryId}`);
        clearInterval(sseRefreshTimer as number);
        sseRefreshTimer = null;
        return;
      }

      // 重新加载详情以获取最新的SSE内容
      console.log(`请求SSE详情更新，ID: ${entryId}`);
      websocketService.requestRequestDetails(entryId);
      websocketService.requestResponseDetails(entryId);
    } else {
      // 如果不再查看SSE请求，停止刷新
      console.log(`不再查看SSE请求，停止刷新，ID: ${entryId}`);
      clearInterval(sseRefreshTimer as number);
      sseRefreshTimer = null;
    }
  }, 1000) as unknown as number;
};

// 停止SSE详情刷新
const stopSSERefresh = () => {
  if (sseRefreshTimer !== null) {
    clearInterval(sseRefreshTimer);
    sseRefreshTimer = null;
  }
};

// 清理定时器
onUnmounted(() => {
  document.removeEventListener('mousemove', handleMouseMove);
  document.removeEventListener('mouseup', stopResize);

  // 清除SSE刷新定时器
  stopSSERefresh();

  // 清除连接检查定时器
  if (connectionCheckTimer.value !== null) {
    clearInterval(connectionCheckTimer.value);
    connectionCheckTimer.value = null;
  }
});

// 设置WebSocket处理器以响应详情
const setupWebSocketDetailHandlers = () => {
  // 请求详情处理
  websocketService.onRequestDetails((details) => {
    store.commit('setRequestDetails', details);
    // 如果正在等待详情加载，则标记为已完成
    if (detailLoading.value) {
      detailLoading.value = false;
    }
  });

  // 响应详情处理
  websocketService.onResponseDetails((details) => {
    store.commit('setResponseDetails', details);
    // 如果正在等待详情加载，则标记为已完成
    if (detailLoading.value) {
      detailLoading.value = false;
    }
  });
};

// 监视连接状态变化
watch(() => isConnected.value, (newValue) => {
  if (newValue) {
    // 连接成功，清除错误提示
    store.commit('setError', null);

    // 如果页面上没有数据，刷新数据
    if (trafficEntries.value.length === 0) {
      refreshData();
    }
  } else {
    // 连接断开，设置错误提示，除非是主动断开
    if (!reconnecting.value) {
      store.commit('setError', 'WebSocket连接已断开，正在尝试重连...');
    }
  }
});

// 刷新数据
const refreshData = () => {
  if (websocketService.isConnected()) {
    store.commit('setLoading', true); // 在获取数据前设置 loading 为 true
    store.dispatch('fetchTrafficEntries').then(() => {
      store.commit('setLoading', false); // 在数据获取完成后设置 loading 为 false
    }).catch(() => {
      store.commit('setLoading', false); // 在数据获取失败时也设置 loading 为 false
    });
  } else {
    // 如果未连接，先尝试重新连接
    console.log('WebSocket未连接，尝试重新连接后再获取数据');
    store.commit('setLoading', true);

    // 给连接一些时间
    setTimeout(() => {
      // 如果连接成功，获取数据
      if (websocketService.isConnected()) {
        store.dispatch('fetchTrafficEntries').then(() => {
          store.commit('setLoading', false);
        }).catch(() => {
          store.commit('setLoading', false);
        });
      } else {
        // 连接仍然失败，显示错误
        store.commit('setError', 'WebSocket连接失败，无法获取数据');
        store.commit('setLoading', false);
      }
    }, 2000);
  }
};

// 清空数据
const confirmClear = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要清空所有流量数据吗？此操作不可恢复。',
      '警告',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    );
    store.dispatch('clearTrafficEntries');
  } catch {
    // 用户取消操作
  }
};

// 修改handleRowClick函数，增加SSE检测逻辑
const handleRowClick = (row: TrafficEntry) => {
  // 防止重复点击
  if (clickDebounceTimer.value) {
    clearTimeout(clickDebounceTimer.value);
  }

  clickDebounceTimer.value = setTimeout(() => {
    if (lastClickedId.value !== row.id) {
      lastClickedId.value = row.id;
      // 开始加载详情
      detailLoading.value = true;
      // 设置被选中的条目
      store.commit('setSelectedEntry', row);

      // 请求详情
      websocketService.requestRequestDetails(row.id);
      websocketService.requestResponseDetails(row.id);

      // 如果是SSE请求，启动刷新定时器
      if (row.isSSE) {
        startSSERefresh(row.id);
      } else {
        // 如果不是SSE请求，确保停止任何现有的刷新定时器
        stopSSERefresh();
      }
    }
  }, 200);
};
</script>

<style>
.traffic-list-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.toolbar {
  padding: 5px 10px;
  background-color: #f5f7fa;
  border-bottom: 1px solid #e6e6e6;
  height: 38px;
  display: flex;
  align-items: center;
}

.toolbar .el-button {
  margin-right: 5px;
}

.connection-status {
  margin-left: auto;
  margin-right: 10px;
}

.error-message {
  max-width: 300px;
  margin-right: 10px;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: height 0.1s;
}

/* 可拖动分隔条 */
.resizer {
  height: 6px;
  background-color: #e0e0e0;
  cursor: ns-resize;
  position: relative;
  user-select: none;
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 10;
}

.resizer::before {
  content: "";
  height: 2px;
  width: 30px;
  background-color: #a0a0a0;
  border-radius: 1px;
}

.resizer:hover, .resizer.resizing {
  background-color: #ccc;
}

.resizer:hover::before, .resizer.resizing::before {
  background-color: #666;
}

.detail-panel {
  overflow: hidden;
  border-top: 1px solid #e6e6e6;
  background-color: #f9f9f9;
  transition: height 0.1s;
}

.status-success {
  color: #67c23a;
}

.status-redirect {
  color: #e6a23c;
}

.status-client-error {
  color: #f56c6c;
}

.status-server-error {
  color: #f56c6c;
  font-weight: bold;
}

.el-table--small {
  font-size: 12px;
}

/* 减小表格行高 */
.el-table th.el-table__cell {
  padding: 5px 0;
}

.el-table__body td.el-table__cell {
  padding: 3px 0;
}

/* 自定义表格样式，使其更接近Burp */
.el-table {
  --el-table-border-color: #dcdfe6;
  --el-table-header-background-color: #ebeef5;
}

.el-table--striped .el-table__body tr.el-table__row--striped td.el-table__cell {
  background-color: #f5f7fa;
}

.el-table .el-table__body tr.current-row > td.el-table__cell {
  background-color: #e6f7ff;
}

.burp-style-table {
  font-family: monospace;
  font-size: 11px;
  border: 1px solid #dcdfe6;
}

.burp-style-table .el-table__header-wrapper th {
  background-color: #f0f0f0;
  font-weight: bold;
  color: #303133;
  height: 30px;
  line-height: 30px;
  border-bottom: 1px solid #dcdfe6;
}

.burp-style-table .el-table__body td {
  border-bottom: 1px solid #ebeef5;
}

.method-cell {
  font-weight: bold;
}

.path-cell {
  color: #606266;
}

/* 修改鼠标样式，提高拖动时的用户体验 */
body.resizing {
  cursor: ns-resize !important;
}
</style>