import { createRouter, createWebHistory } from 'vue-router'
import TranslateRecords from '../views/TranslateRecords.vue'
 
const routes = [
    {
        path: '/',
        name: 'TranslateRecords',
        component: TranslateRecords
    },
]

const router = createRouter({
    history: createWebHistory(),
    routes
})
export default router