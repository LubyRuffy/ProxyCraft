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
          <div class="panel-header">
            <div class="panel-title">请求</div>
            <div class="panel-actions">
              <el-radio-group v-model="requestView" size="small" class="view-toggle">
                <el-radio-button label="pretty">美化</el-radio-button>
                <el-radio-button label="raw">原始</el-radio-button>
                <el-radio-button label="hex">十六进制</el-radio-button>
              </el-radio-group>
              <el-dropdown size="small" trigger="click" class="view-options">
                <el-button size="small" :icon="Setting" circle></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item>自动换行</el-dropdown-item>
                    <el-dropdown-item>高亮语法</el-dropdown-item>
                    <el-dropdown-item>URL解码</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>

          <div v-if="request" class="panel-body">
            <!-- 请求信息行 -->
            <div class="request-line">
              <span class="method">{{ requestMethod }}</span>
              <span class="url">{{ requestUrl }}</span>
              <span class="protocol">{{ requestProtocol }}</span>
            </div>

            <!-- 请求头部 -->
            <div class="headers-container">
              <div v-for="(value, name) in request.headers" :key="name" class="header-line">
                <span class="header-name">{{ name }}:</span>
                <span class="header-value">{{ value }}</span>
              </div>
            </div>

            <!-- 请求体 -->
            <div v-if="requestHasBody" class="body-container">
              <div v-if="requestView === 'pretty'" class="pretty-view">
                <pre v-if="typeof request.body === 'string' && isJson(request.body)" v-html="formatJson(request.body)"></pre>
                <pre v-else-if="typeof request.body === 'string'">{{ request.body }}</pre>
                <pre v-else v-html="formatJson(request.body)"></pre>
              </div>
              <div v-else-if="requestView === 'raw'" class="raw-view">
                <pre>{{ typeof request.body === 'string' ? request.body : JSON.stringify(request.body) }}</pre>
              </div>
              <div v-else-if="requestView === 'hex'" class="hex-view">
                <div class="hex-dump">
                  {{ getHexView(request.body) }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          class="panel-divider"
          v-show="displayMode === 'split'"
          @mousedown="startResize"
        ></div>

        <div class="response-pane" v-show="displayMode === 'split' || displayMode === 'response'" :style="{ flex: responseFlex }">
          <div class="panel-header">
            <div class="panel-title">响应</div>
            <div class="panel-actions">
              <el-radio-group v-model="responseView" size="small" class="view-toggle">
                <el-radio-button label="pretty">美化</el-radio-button>
                <el-radio-button label="raw">原始</el-radio-button>
                <el-radio-button label="hex">十六进制</el-radio-button>
              </el-radio-group>
              <el-dropdown size="small" trigger="click" class="view-options">
                <el-button size="small" :icon="Setting" circle></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item>自动换行</el-dropdown-item>
                    <el-dropdown-item>高亮语法</el-dropdown-item>
                    <el-dropdown-item>URL解码</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>

          <div v-if="response" class="panel-body">
            <!-- 响应状态行 -->
            <div class="response-line">
              <span class="protocol">{{ responseProtocol }}</span>
              <span :class="['status-code', getStatusClass(responseStatusCode)]">{{ responseStatusCode }}</span>
              <span class="status-text">{{ getStatusText(responseStatusCode) }}</span>
            </div>

            <!-- 响应头部 -->
            <div class="headers-container">
              <div v-for="(value, name) in response.headers" :key="name" class="header-line">
                <span class="header-name">{{ name }}:</span>
                <span class="header-value">{{ value }}</span>
              </div>
            </div>

            <!-- 响应体 -->
            <div v-if="responseHasBody" class="body-container">
              <div v-if="responseView === 'pretty'" class="pretty-view">
                <pre v-if="typeof response.body === 'string' && isJson(response.body)" v-html="formatJson(response.body)"></pre>
                <pre v-else-if="typeof response.body === 'string'">{{ response.body }}</pre>
                <pre v-else v-html="formatJson(response.body)"></pre>
              </div>
              <div v-else-if="responseView === 'raw'" class="raw-view">
                <pre>{{ typeof response.body === 'string' ? response.body : JSON.stringify(response.body) }}</pre>
              </div>
              <div v-else-if="responseView === 'hex'" class="hex-view">
                <div class="hex-dump">
                  {{ getHexView(response.body) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import { RequestDetails, ResponseDetails, TrafficEntry } from '../store';
import { Search, ArrowUp, ArrowDown, Setting, Share, CopyDocument, Download } from '@element-plus/icons-vue';
import hljs from 'highlight.js/lib/core';
import json from 'highlight.js/lib/languages/json';
import 'highlight.js/styles/github.css';

// 只注册需要的语言以减小打包体积
hljs.registerLanguage('json', json);

const props = defineProps<{
  request: RequestDetails | null;
  response: ResponseDetails | null;
  loading: boolean;
  selectedEntry: TrafficEntry | null;
}>();

const requestView = ref('pretty');
const responseView = ref('pretty');
const displayMode = ref('split');
const searchKeyword = ref('');
const copyStatus = ref<'success' | 'error' | null>(null);

// 分隔条拖动相关状态
const isResizing = ref(false);
const requestFlex = ref('1');
const responseFlex = ref('1');
const startX = ref(0);
const startRequestWidth = ref(0);
const startResponseWidth = ref(0);

// 处理显示模式改变后的逻辑
const handleDisplayModeChange = () => {
  // 重置分隔条相关状态
  isResizing.value = false;
  requestFlex.value = '1';
  responseFlex.value = '1';
  startX.value = 0;
  startRequestWidth.value = 0;
  startResponseWidth.value = 0;
};

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

// 提取请求信息
const requestMethod = computed(() => props.selectedEntry?.method || 'GET');
const requestUrl = computed(() => props.selectedEntry?.path || '/');
const requestProtocol = computed(() => {
  return props.selectedEntry?.protocol || 'HTTP/1.1';
});
const requestHasBody = computed(() => {
  // 检查是否存在请求体，且请求体不为空
  if (!props.request?.body) return false;

  // 对于字符串类型的请求体，检查长度是否大于0
  if (typeof props.request.body === 'string') {
    // 如果是Binary data的情况，不显示
    if (props.request.body === '<Binary data, 0 bytes>') {
      return false;
    }
    return props.request.body.length > 0;
  }

  // 对于对象类型的请求体，检查是否为空对象或有内容
  if (typeof props.request.body === 'object') {
    // 如果是Binary data的情况，不显示
    if (props.request.body === '<Binary data, 0 bytes>') {
      return false;
    }

    // 检查是否是空对象
    return Object.keys(props.request.body).length > 0;
  }

  return true;
});

// 提取响应信息
const responseStatusCode = computed(() => props.selectedEntry?.statusCode || 0);
const responseProtocol = computed(() => {
  return props.selectedEntry?.protocol || 'HTTP/1.1';
});
const responseHasBody = computed(() => {
  // 检查是否存在响应体，且响应体不为空
  if (!props.response?.body) return false;

  // 对于字符串类型的响应体，检查长度是否大于0
  if (typeof props.response.body === 'string') {
    // 如果是二进制数据的情况，特别处理
    if (props.response.body === '<Binary data, 0 bytes>') {
      return false;
    }
    return props.response.body.length > 0;
  }

  // 对于对象类型的响应体，检查是否为空对象或有内容
  if (typeof props.response.body === 'object') {
    return Object.keys(props.response.body).length > 0;
  }

  return true;
});

// 获取状态码的CSS类
const getStatusClass = (statusCode: number) => {
  if (statusCode >= 200 && statusCode < 300) {
    return 'status-success';
  } else if (statusCode >= 300 && statusCode < 400) {
    return 'status-redirect';
  } else if (statusCode >= 400 && statusCode < 500) {
    return 'status-client-error';
  } else if (statusCode >= 500) {
    return 'status-server-error';
  }
  return '';
};

// 获取状态码的文本描述
const getStatusText = (statusCode: number) => {
  const statusTexts: Record<number, string> = {
    200: 'OK',
    201: 'Created',
    204: 'No Content',
    301: 'Moved Permanently',
    302: 'Found',
    304: 'Not Modified',
    400: 'Bad Request',
    401: 'Unauthorized',
    403: 'Forbidden',
    404: 'Not Found',
    500: 'Internal Server Error',
    502: 'Bad Gateway',
    503: 'Service Unavailable',
    504: 'Gateway Timeout'
  };

  return statusTexts[statusCode] || '';
};


// 获取十六进制视图
const getHexView = (data: any): string => {
  if (!data) return '';

  // 将数据转换为字符串
  let str = typeof data === 'string' ? data : JSON.stringify(data);

  // 将字符串转换为UTF-8字节数组
  let bytes = new TextEncoder().encode(str);
  let result = '';

  // 每16个字节为一行
  for (let i = 0; i < bytes.length; i += 16) {
    // 添加偏移量
    result += (i).toString(16).padStart(8, '0') + ': ';

    let hexPart = '';
    let asciiPart = '';

    // 处理这一行的字节
    for (let j = 0; j < 16; j++) {
      if (i + j < bytes.length) {
        // 添加十六进制表示，保持两位宽度
        hexPart += bytes[i + j].toString(16).padStart(2, '0') + ' ';
        // 添加ASCII表示
        const byte = bytes[i + j];
        asciiPart += (byte >= 32 && byte <= 126) ? String.fromCharCode(byte) : '.';
      } else {
        // 用空格填充未满16字节的行
        hexPart += '   ';
      }
    }

    // 确保十六进制部分对齐，添加到结果中
    result += hexPart + ' ' + asciiPart;

    // 如果不是最后一行，添加换行符
    if (i + 16 < bytes.length) {
      result += '\n';
    }
  }

  return result;
};

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
  if (requestHasBody.value && props.request.body) {
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

  return curlCommand;
};

/**
 * 复制cURL命令到剪贴板
 */
// 检查字符串是否为JSON格式
const isJson = (str: string): boolean => {
  try {
    JSON.parse(str);
    return true;
  } catch (e) {
    return false;
  }
};

// 格式化并高亮JSON
const formatJson = (content: string | any): string => {
  try {
    // 如果输入是JSON字符串，先解析再格式化
    const jsonObj = typeof content === 'string' ? JSON.parse(content) : content;
    const formatted = JSON.stringify(jsonObj, null, 2);
    // 使用pre标签包装，确保正确应用hljs的样式
    const highlighted = hljs.highlight(formatted, { language: 'json' }).value;
    return `<code class="hljs language-json">${highlighted}</code>`;
  } catch (e) {
    console.error('JSON格式化失败:', e);
    return String(content);
  }
};

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

.search-bar {
  display: flex;
  align-items: center;
}

.search-bar .el-input {
  width: 200px;
}

.search-bar .el-button {
  margin-left: 5px;
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
  background-color: white;
  border: 1px solid #dcdfe6;
  margin: 5px;
  border-radius: 3px;
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

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 10px;
  background-color: #f5f7fa;
  border-bottom: 1px solid #e6e6e6;
  height: 32px;
}

.panel-title {
  font-weight: bold;
  font-size: 12px;
  color: #303133;
}

.panel-actions {
  display: flex;
  align-items: center;
}

.view-toggle {
  margin-right: 10px;
}

.view-options {
  margin-left: 5px;
}

.panel-body {
  flex: 1;
  overflow: auto;
  padding: 5px;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.4;
}

.request-line, .response-line {
  padding: 5px;
  background-color: #f5f7fa;
  margin-bottom: 5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-radius: 3px;
}

.method {
  font-weight: bold;
  margin-right: 10px;
  color: #409eff;
}

.url {
  color: #333;
}

.protocol {
  color: #909399;
  margin-left: 10px;
}

.status-code {
  font-weight: bold;
  margin: 0 10px;
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
}

.status-text {
  color: #333;
}

.headers-container {
  padding: 0 5px;
  margin-bottom: 10px;
  border-bottom: 1px dashed #dcdfe6;
}

.header-line {
  line-height: 1.4;
  margin-bottom: 2px;
}

.header-name {
  color: #409eff;
  margin-right: 5px;
}

.header-value {
  color: #333;
}

.body-container {
  padding: 5px;
}

.pretty-view, .raw-view, .hex-view {
  padding: 5px;
  background-color: #f9f9f9;
  border-radius: 3px;
  overflow: auto;
}

/* JSON高亮样式覆盖 */
.pretty-view pre {
  background-color: transparent !important;
}

/* highlight.js的JSON语法高亮样式 */
.pretty-view .hljs {
  color: #24292e;
  background: transparent;
  padding: 0;
}

.pretty-view .hljs-attr {
  color: #005cc5;
}

.pretty-view .hljs-string {
  color: #22863a;
}

.pretty-view .hljs-number {
  color: #005cc5;
}

.pretty-view .hljs-literal {
  color: #005cc5;
}

.pretty-view .hljs-punctuation {
  color: #24292e;
}

.pretty-view .hljs-comment {
  color: #6a737d;
}

/* 确保pre和code标签正确显示 */
.pretty-view pre {
  margin: 0;
  padding: 0;
  background: transparent;
}

.pretty-view code {
  font-family: Monaco, Menlo, Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.hex-dump {
  white-space: pre;
  font-family: monospace;
}

pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}
</style> 