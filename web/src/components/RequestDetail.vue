<template>
  <div class="request-detail" v-loading="loading">
    <div v-if="!request">
      <el-empty description="请选择一个请求查看详情" :image-size="60" />
    </div>
    <div v-else>
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

      <div class="panel-body">
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
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { RequestDetails, TrafficEntry } from '../store';
import { Setting } from '@element-plus/icons-vue';
import { isJson, formatJson, getHexView } from '../utils/formatters';

const props = defineProps<{
  request: RequestDetails | null;
  loading: boolean;
  selectedEntry: TrafficEntry | null;
}>();

const requestView = ref('pretty');

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
</script>

<style>
.request-detail {
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

.request-line {
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