<template>
  <div class="app-container">
    <router-view></router-view>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'App',
});
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: Arial, sans-serif;
  font-size: 12px;
}

.app-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

/* 全局紧凑型样式覆盖 */
.el-button {
  padding: 5px 10px;
  font-size: 12px;
  height: 28px;
}

.el-input__inner {
  height: 28px;
  line-height: 28px;
  font-size: 12px;
}

.el-table {
  font-size: 12px;
}

.el-table td, .el-table th {
  padding: 5px 0;
}

.el-descriptions__label, .el-descriptions__content {
  padding: 5px !important;
  font-size: 12px;
}

.el-tabs__item {
  height: 30px;
  line-height: 30px;
  font-size: 12px;
  padding: 0 10px;
}

.el-tab-pane {
  padding: 5px;
}

.el-dialog__title {
  font-size: 14px;
}

.el-dialog__body {
  padding: 10px;
}

/* 禁用所有动画以获得更好的性能 */
* {
  transition: none !important;
  animation: none !important;
}
</style> 