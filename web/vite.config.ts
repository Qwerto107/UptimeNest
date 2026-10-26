import { fileURLToPath, URL } from "node:url";
import { execFileSync } from "node:child_process";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

function commitHash() {
  const supplied = process.env.BUILD_COMMIT || process.env.GITHUB_SHA;
  if (supplied && /^[a-f0-9]{7,64}$/i.test(supplied)) return supplied.slice(0, 7).toLowerCase();
  // 源码包和 Docker 构建可能没有 Git，缺失提交信息时仍允许正常构建。
  try {
    return execFileSync("git", ["rev-parse", "--short=7", "HEAD"], {
      cwd: fileURLToPath(new URL("..", import.meta.url)),
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      timeout: 1000,
    }).trim();
  } catch {
    return "unknown";
  }
}

export default defineConfig({
  define: { "import.meta.env.VITE_COMMIT_HASH": JSON.stringify(commitHash()) },
  plugins: [vue(), tailwindcss()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    proxy: { "/api": { target: "http://127.0.0.1:8090", changeOrigin: false } },
  },
});
