import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router';
import TrafficList from '../views/TrafficList.vue';

const routes: Array<RouteRecordRaw> = [
  {
    path: '/',
    name: 'TrafficList',
    component: TrafficList,
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router; 