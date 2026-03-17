<template>
  <el-row class="tac" style="height: 100%;">
    <el-col :span="24">
      <el-menu
        class="el-menu-vertical-demo"
        default-active="/"
        @open="handleOpen"
        @close="handleClose"
      router>
        <el-menu-item index="/">
          <el-icon><VideoPlay /></el-icon>
          <span>视频列表</span>
        </el-menu-item>
      
      </el-menu>
    </el-col>
  </el-row>
</template>

<script lang="ts" setup>
import { VideoPlay } from '@element-plus/icons-vue'

const handleOpen = (key: string, keyPath: string[]) => {
  console.log(key, keyPath)
}
const handleClose = (key: string, keyPath: string[]) => {
  console.log(key, keyPath)
}
</script>