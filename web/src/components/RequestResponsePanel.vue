<template>
  <div class="request-response-panel" v-loading="loading">
    <div v-if="!request && !response">
      <el-empty description="请选择一个请求查看详情" :image-size="60" />
    </div>
    <div v-else class="panel-container">
      <!-- 顶部工具栏 -->
      <div class="top-toolbar">
        <!-- 切换显示方式的按钮 -->
        <el-radio-group v-model="displayMode" size="small">
          <el-radio-button label="split">并排显示</el-radio-button>
          <el-radio-button label="request">仅请求</el-radio-button>
          <el-radio-button label="response">仅响应</el-radio-button>
        </el-radio-group>

        <!-- 操作按钮 -->
        <div class="action-buttons">
          <span v-if="copyStatus" class="copy-status" :class="{ 'copy-success': copyStatus === 'success', 'copy-error': copyStatus === 'error' }">
            {{ copyStatus === 'success' ? '已复制' : '复制失败' }}
          </span>
          <el-tooltip content="复制为curl命令" placement="top">
            <el-button size="small" :icon="CopyDocument" circle @click="copyAsCurl"></el-button>
          </el-tooltip>
        </div>
      </div>

      <!-- 分隔条 -->
      <div class="splitter" :class="{'full-request': displayMode === 'request', 'full-response': displayMode === 'response'}">
        <div class="request-pane" v-show="displayMode === 'split' || displayMode === 'request'" :style="{ flex: requestFlex }">
          <request-detail
            :request="request"
            :loading="loading"
            :selectedEntry="selectedEntry"
          />
        </div>

        <div
          class="panel-divider"
          v-show="displayMode === 'split'"
          @mousedown="startResize"
        ></div>

        <div class="response-pane" v-show="displayMode === 'split' || displayMode === 'response'" :style="{ flex: responseFlex }">
          <response-detail
            :response="response"
            :loading="loading"
            :selectedEntry="selectedEntry"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue';
import { RequestDetails, ResponseDetails, TrafficEntry } from '../store';
import { CopyDocument } from '@element-plus/icons-vue';
import RequestDetail from './RequestDetail.vue';
import ResponseDetail from './ResponseDetail.vue';

const props = defineProps<{
  request: RequestDetails | null;
  response: ResponseDetails | null;
  loading: boolean;
  selectedEntry: TrafficEntry | null;
}>();

const displayMode = ref('split');
const copyStatus = ref<'success' | 'error' | null>(null);

// 分隔条拖动相关状态
const isResizing = ref(false);
const requestFlex = ref('1');
const responseFlex = ref('1');
const startX = ref(0);
const startRequestWidth = ref(0);
const startResponseWidth = ref(0);

// 开始拖动
const startResize = (e: MouseEvent) => {
  isResizing.value = true;
  startX.value = e.clientX;

  // 获取当前两个面板的宽度
  const requestPane = document.querySelector('.request-pane') as HTMLElement;
  const responsePane = document.querySelector('.response-pane') as HTMLElement;

  if (requestPane && responsePane) {
    startRequestWidth.value = requestPane.offsetWidth;
    startResponseWidth.value = responsePane.offsetWidth;
  }

  // 添加全局鼠标事件监听
  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', stopResize);

  // 添加防止选择文本的样式
  document.body.style.userSelect = 'none';
};

// 拖动中
const onMouseMove = (e: MouseEvent) => {
  if (!isResizing.value) return;

  const deltaX = e.clientX - startX.value;
  const totalWidth = startRequestWidth.value + startResponseWidth.value;

  const newRequestWidth = startRequestWidth.value + deltaX;
  const newResponseWidth = startResponseWidth.value - deltaX;

  // 确保不会拖到特别小的尺寸
  if (newRequestWidth < 100 || newResponseWidth < 100) return;

  // 设置flex比例
  requestFlex.value = `${newRequestWidth / totalWidth}`;
  responseFlex.value = `${newResponseWidth / totalWidth}`;
};

// 停止拖动
const stopResize = () => {
  isResizing.value = false;
  document.removeEventListener('mousemove', onMouseMove);
  document.removeEventListener('mouseup', stopResize);
  document.body.style.userSelect = '';
};

// 在组件卸载前移除事件监听
onBeforeUnmount(() => {
  if (isResizing.value) {
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', stopResize);
    document.body.style.userSelect = '';
  }
});

/**
 * 生成cURL命令
 */
const generateCurlCommand = (): string => {
  if (!props.request || !props.selectedEntry) return '';

  const { method, path, protocol, host, host_with_schema } = props.selectedEntry;

  // 基本的cURL命令
  let curlCommand = `curl -X ${method}`;

  // 添加URL
  let url = `${host_with_schema.toLowerCase()}${path}`;
  if (props.request.headers.Host) {
    url += `--resolve ${props.request.headers.Host}:443`;
  }
  curlCommand += ` "${url}"`;

  // 添加请求头
  if (props.request?.headers) {
    for (const [name, value] of Object.entries(props.request.headers)) {
      if (name.toLowerCase() !== 'host') { // 跳过Host，因为已经在URL中处理了
        curlCommand += ` -H "${name}: ${value}"`;
      }
    }
  }

  // 添加请求体（如果有）
  if (props.request.body) {
    // 检查请求体是否为空
    let hasBody = false;

    if (typeof props.request.body === 'string') {
      // 如果是Binary data的情况，不显示
      if (props.request.body !== '<Binary data, 0 bytes>' && props.request.body.length > 0) {
        hasBody = true;
      }
    } else if (typeof props.request.body === 'object' && props.request.body !== null) {
      // 检查是否是空对象
      if (Object.keys(props.request.body).length > 0) {
        hasBody = true;
      }
    }

    if (hasBody) {
      let bodyContent = '';

      if (typeof props.request.body === 'string') {
        bodyContent = props.request.body;
      } else {
        try {
          bodyContent = JSON.stringify(props.request.body);
        } catch (e) {
          console.error('无法将请求体转换为JSON:', e);
          return '';
        }
      }

      curlCommand += ` --data '${bodyContent}'`;
    }
  }

  return curlCommand;
};

/**
 * 复制cURL命令到剪贴板
 */
const copyAsCurl = async () => {
  const curlCommand = generateCurlCommand();
  if (!curlCommand) {
    copyStatus.value = 'error';
    setTimeout(() => copyStatus.value = null, 3000);
    return;
  }

  try {
    await navigator.clipboard.writeText(curlCommand);
    copyStatus.value = 'success';
    setTimeout(() => copyStatus.value = null, 3000);
  } catch (err) {
    console.error('复制失败:', err);
    copyStatus.value = 'error';
    setTimeout(() => copyStatus.value = null, 3000);
  }
};
</script>

<style>
.request-response-panel {
  height: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-container {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: #f9f9f9;
}

.top-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 10px;
  background-color: #f0f0f0;
  border-bottom: 1px solid #e6e6e6;
}

.action-buttons {
  display: flex;
  gap: 8px;
  align-items: center;
}

.copy-status {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
  transition: opacity 0.3s ease;
}

.copy-success {
  color: #67c23a;
  background-color: #f0f9eb;
}

.copy-error {
  color: #f56c6c;
  background-color: #fef0f0;
}

.splitter {
  display: flex;
  flex: 1;
  height: calc(100% - 40px);
  overflow: hidden;
}

.splitter.full-request {
  display: flex;
  flex-direction: column;
}

.splitter.full-request .request-pane {
  flex: 1 !important;
  width: 100% !important;
}

.splitter.full-request .response-pane {
  display: none;
}

.splitter.full-response {
  display: flex;
  flex-direction: column;
}

.splitter.full-response .response-pane {
  flex: 1 !important;
  width: 100% !important;
}

.splitter.full-response .request-pane {
  display: none;
}

.request-pane, .response-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  margin: 5px;
}

.panel-divider {
  width: 5px;
  background-color: #f0f0f0;
  cursor: col-resize;
  transition: background-color 0.2s;
  margin: 5px 0;
}

.panel-divider:hover {
  background-color: #c0c4cc;
}
</style>