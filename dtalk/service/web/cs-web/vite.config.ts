import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

/**
 * 代理到网关或 CS 直连（浏览器请求 /api/v1/...，转发时去掉 /api/v1）。
 *
 * DeepSeek：浏览器请求 `/deepseek-proxy/v1/...`，开发环境下由本代理转发至 api.deepseek.com，
 * 并在服务端附加 Authorization（读取 `.env.local` / `.env` 中的 DEEPSEEK_API_KEY）。
 *
 * - 默认仍为线上地址；若出现 `/cs/chain/*` 404，说明该网关未部署含新路由的版本。
 * - 本地开发：先 `go run` 本仓库编译出的 gateway（或仅跑 CS 服务 18030，仅 CS 接口可用）。
 *
 * 示例：
 *   GATEWAY_PROXY_TARGET=http://127.0.0.1:8888 npm run dev
 * 仅连本机 CS（客服登录/链绑定，不含 /store 等需网关转发的接口）：
 *   GATEWAY_PROXY_TARGET=http://127.0.0.1:18030 npm run dev
 */
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const gatewayTarget =
    process.env.GATEWAY_PROXY_TARGET || env.GATEWAY_PROXY_TARGET || 'http://localhost:8888';
  const deepseekKey = env.DEEPSEEK_API_KEY || process.env.DEEPSEEK_API_KEY || '';

  if (mode === 'development' && !deepseekKey) {
    console.warn(
      '[vite] 未检测到 DEEPSEEK_API_KEY（可在 cs-web/.env.local 配置）。AI 自动客服请求将返回 401。',
    );
  }

  return {
    plugins: [react()],
    server: {
      port: 3000,
      // 默认仅绑定 localhost，局域网/公网 IP 无法访问；设为 true 等价于监听 0.0.0.0
      host: true,
      proxy: {
        '/api': {
          target: gatewayTarget,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/v1/, '') || '/',
        },
        '/deepseek-proxy': {
          target: 'https://api.deepseek.com',
          changeOrigin: true,
          secure: true,
          rewrite: (path) => path.replace(/^\/deepseek-proxy/, ''),
          configure: (proxy) => {
            proxy.on('proxyReq', (proxyReq) => {
              if (deepseekKey) {
                proxyReq.setHeader('Authorization', `Bearer ${deepseekKey}`);
              }
            });
          },
        },
      },
    },
  };
});
