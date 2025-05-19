<template>
  <div class="request-detail" v-loading="loading">
    <div v-if="!request">
      <el-empty description="请选择一个请求查看详情" :image-size="60" />
    </div>
    <div v-else>
      <el-tabs type="card" class="compact-tabs">
        <el-tab-pane label="头部">
          <div class="headers-container">
            <el-descriptions border :column="1" size="small">
              <el-descriptions-item v-for="(value, name) in request.headers" :key="name" :label="name">
                {{ value }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </el-tab-pane>
        <el-tab-pane label="内容">
          <div class="body-container">
            <pre v-if="typeof request.body === 'string'">{{ request.body }}</pre>
            <pre v-else>{{ JSON.stringify(request.body, null, 2) }}</pre>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, PropType } from 'vue';
import { RequestDetails } from '../store';

export default defineComponent({
  name: 'RequestDetail',
  props: {
    request: {
      type: Object as PropType<RequestDetails>,
      required: false,
      default: null
    },
    loading: {
      type: Boolean,
      required: false,
      default: false
    }
  }
});
</script>

<style>
.request-detail {
  height: 100%;
  overflow: auto;
}

.headers-container {
  padding: 5px;
  overflow: auto;
  max-height: 180px;
}

.body-container {
  padding: 5px;
  overflow: auto;
  max-height: 180px;
  background-color: #f9f9f9;
}

pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: monospace;
  font-size: 11px;
  line-height: 1.3;
}

.compact-tabs .el-tabs__header {
  margin-bottom: 5px;
}

.compact-tabs .el-tabs__nav-wrap {
  padding: 0;
}
</style> 