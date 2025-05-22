<template>
  <div class="response-detail" v-loading="loading">
    <div v-if="!response">
      <el-empty description="请选择一个请求查看响应详情" :image-size="60" />
    </div>
    <div v-else>
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

      <div class="panel-body">
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
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { ResponseDetails, TrafficEntry } from '../store';
import { Setting } from '@element-plus/icons-vue';
import { isJson, formatJson, getHexView, getStatusClass, getStatusText } from '../utils/formatters';

const props = defineProps<{
  response: ResponseDetails | null;
  loading: boolean;
  selectedEntry: TrafficEntry | null;
}>();

const responseView = ref('pretty');

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
</script>

<style>
.response-detail {
  height: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background-color: white;
  border: 1px solid #dcdfe6;
  border-radius: 3px;
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

.response-line {
  padding: 5px;
  background-color: #f5f7fa;
  margin-bottom: 5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-radius: 3px;
}

.protocol {
  color: #909399;
  margin-right: 10px;
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
  word-wrap: break-word;
  font-family: monospace;
  font-size: 11px;
  line-height: 1.3;
}
</style>