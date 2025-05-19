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
        
        <!-- 搜索框 -->
        <div class="search-bar">
          <el-input
            v-model="searchKeyword"
            placeholder="查找"
            size="small"
            :prefix-icon="Search"
            clearable
            @clear="clearSearch"
            @keyup.enter="performSearch"
          />
          <el-button size="small" :icon="ArrowUp" @click="searchPrevious" :disabled="!searchKeyword"></el-button>
          <el-button size="small" :icon="ArrowDown" @click="searchNext" :disabled="!searchKeyword"></el-button>
        </div>
        
        <!-- 操作按钮 -->
        <div class="action-buttons">
          <el-tooltip content="发送到重放器" placement="top">
            <el-button size="small" :icon="Share" circle></el-button>
          </el-tooltip>
          <el-tooltip content="复制为curl命令" placement="top">
            <el-button size="small" :icon="CopyDocument" circle></el-button>
          </el-tooltip>
          <el-tooltip content="导出请求/响应" placement="top">
            <el-button size="small" :icon="Download" circle></el-button>
          </el-tooltip>
        </div>
      </div>
      
      <!-- 分隔条 -->
      <div class="splitter" :class="{'full-request': displayMode === 'request', 'full-response': displayMode === 'response'}">
        <div class="request-pane" v-show="displayMode === 'split' || displayMode === 'request'">
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
                <pre v-if="typeof request.body === 'string'">{{ request.body }}</pre>
                <pre v-else>{{ JSON.stringify(request.body, null, 2) }}</pre>
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
        
        <div class="panel-divider" v-show="displayMode === 'split'"></div>
        
        <div class="response-pane" v-show="displayMode === 'split' || displayMode === 'response'">
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
                <pre v-if="typeof response.body === 'string'">{{ response.body }}</pre>
                <pre v-else>{{ JSON.stringify(response.body, null, 2) }}</pre>
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
import { computed, ref } from 'vue';
import { RequestDetails, ResponseDetails, TrafficEntry } from '../store';
import { Search, ArrowUp, ArrowDown, Setting, Share, CopyDocument, Download } from '@element-plus/icons-vue';

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

// 提取请求信息
const requestMethod = computed(() => props.selectedEntry?.method || 'GET');
const requestUrl = computed(() => props.selectedEntry?.path || '/');
const requestProtocol = computed(() => props.selectedEntry?.isHTTPS ? 'HTTPS' : 'HTTP');
const requestHasBody = computed(() => {
  return props.request?.body && 
          (typeof props.request.body === 'string' ? props.request.body.length > 0 : true);
});

// 提取响应信息
const responseStatusCode = computed(() => props.selectedEntry?.statusCode || 0);
const responseProtocol = computed(() => props.selectedEntry?.isHTTPS ? 'HTTPS' : 'HTTP');
const responseHasBody = computed(() => {
  return props.response?.body && 
          (typeof props.response.body === 'string' ? props.response.body.length > 0 : true);
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
  
  let str = typeof data === 'string' ? data : JSON.stringify(data);
  let result = '';
  let asciiResult = '';
  
  for (let i = 0; i < str.length; i++) {
    // 每16个字符换行
    if (i % 16 === 0 && i !== 0) {
      result += '  ' + asciiResult + '\n';
      asciiResult = '';
    }
    
    // 计算偏移量并添加到行首
    if (i % 16 === 0) {
      result += (i).toString(16).padStart(8, '0') + ': ';
    }
    
    // 获取字符的十六进制表示
    const charCode = str.charCodeAt(i);
    const hex = charCode.toString(16).padStart(2, '0');
    result += hex + ' ';
    
    // 为ASCII表示准备字符
    asciiResult += (charCode >= 32 && charCode <= 126) ? str[i] : '.';
  }
  
  // 处理最后一行的空白和ASCII表示
  const lastLineLength = str.length % 16;
  if (lastLineLength > 0) {
    const padding = 16 - lastLineLength;
    result += '   '.repeat(padding) + '  ' + asciiResult;
  } else if (str.length > 0) {
    result += '  ' + asciiResult;
  }
  
  return result;
};

// 搜索功能
const clearSearch = () => {
  searchKeyword.value = '';
};

const performSearch = () => {
  console.log('搜索:', searchKeyword.value);
  // 实际搜索功能需要实现
};

const searchNext = () => {
  console.log('查找下一个');
  // 实际搜索功能需要实现
};

const searchPrevious = () => {
  console.log('查找上一个');
  // 实际搜索功能需要实现
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
  gap: 5px;
}

.splitter {
  display: flex;
  flex: 1;
  height: calc(100% - 40px);
  overflow: hidden;
}

.splitter.full-request .request-pane {
  flex: 1;
}

.splitter.full-response .response-pane {
  flex: 1;
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
  background-color: #f9f9f9;
  cursor: col-resize;
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