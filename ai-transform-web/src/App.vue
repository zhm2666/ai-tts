<script setup lang="ts">
import Menu from './components/Menu.vue';
</script>

 <template>
  <div class="common-layout container" >
    <el-container class="container">
      <el-header class="header"></el-header>
      <el-container>
        <el-aside class="aside"><Menu/></el-aside>
        <el-main> <router-view /></el-main>
      </el-container>
    </el-container>
  </div>
</template> 

<style scoped>
.logo {
  height: 6em;
  padding: 1.5em;
  will-change: filter;
  transition: filter 300ms;
}
.logo:hover {
  filter: drop-shadow(0 0 2em #646cffaa);
}
.logo.vue:hover {
  filter: drop-shadow(0 0 2em #42b883aa);
}
.container{
  height: 100%;
  width: 100%;
}
.header {
  background-color: #545c64;
}
.aside {
  width: 12rem;
}
</style>
