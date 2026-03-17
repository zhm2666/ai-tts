import axios from "axios";
import { ElMessage } from 'element-plus';
import {getCookieValue} from '../utils/utils'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL
})

// 添加请求拦截器
request.interceptors.request.use(function (config) {
    // 在发送请求之前做些什么
     let access_token = getCookieValue("sso_0voice_access_token")
        if (access_token) {
            config.headers.Authorization = "Bearer " + access_token;
        }else{
            config.headers.Authorization = "Bearer " + "access_token_example";
            //window.location.href = import.meta.env.VITE_USER_CENTER
        }
    return config;
  }, function (error) {
    // 对请求错误做些什么
    return Promise.reject(error);
  });

// 添加响应拦截器
request.interceptors.response.use(function (response) {
    // 2xx 范围内的状态码都会触发该函数。
    // 对响应数据做点什么
    return response;
  }, function (error) {
    // 超出 2xx 范围的状态码都会触发该函数。
    // 对响应错误做点什么
    if (error.response.status == 500) {
        ElMessage({
                showClose: true,
                message: '服务器错误，请稍后重试',
                type: 'error',
              })
    }else if (error.response.status == 400) {
        ElMessage({
                showClose: true,
                message: "无效的输入",
                type: 'error',
              })
    }else if (error.response.status == 401) {
       // window.location.href = import.meta.env.VITE_USER_CENTER 
    }
    return Promise.reject(error);
  });
export default request;
