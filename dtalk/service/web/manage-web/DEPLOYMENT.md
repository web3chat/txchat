# 管理后台部署（manage-web 直连 chatadmin）

## 方式

1. chatadmin 监听 `:18020`
2. manage-web 用 Python 静态服务开在 `:8000`
3. 浏览器打开页面后，JS 直连 `http://127.0.0.1:18020`

> `127.0.0.1` 只在「浏览器所在机器 = chatadmin 所在机器」时有效。  
> 若用电脑浏览器访问服务器上的页面，请把 `config.js` 的 `BASE_URL` 改成服务器可达地址（如 `http://localhost:18020`），并在 `chatadmin.toml` 的 `[Cors].AllowOrigins` 里加上页面 Origin（如 `http://localhost:8000`）。

---

## 1. 启动 / 更新 chatadmin

```bash
cd /path/to/dtalk
make update MODULES="chatadmin" REFRESH_CONFIG=1
```

编辑配置卷里的 `chatadmin.toml`（或仓库 `service/chatadmin/config/chatadmin.toml` 后带 `REFRESH_CONFIG=1` 更新）：

```toml
[Cors]
AllowOrigins = [
    "http://127.0.0.1:8000",
    "http://localhost:8000",
]
AllowCredentials = true
```

重启 chatadmin 使 CORS 生效。

---

## 2. 启动 manage-web

```bash
cd /path/to/dtalk/service/web/manage-web
# 确认
grep BASE_URL config.js
# BASE_URL: 'http://127.0.0.1:18020'

python3 -m http.server 8000
# 或 ./start.sh
```

浏览器打开：`http://127.0.0.1:8000`

若曾改过 API 地址，控制台执行：

```js
localStorage.removeItem('apiBaseURL');
location.reload();
```

---

## 3. 端口

| 服务 | 端口 |
|------|------|
| manage-web | 8000 |
| chatadmin | 18020 |

防火墙需允许浏览器访问这两个端口（同机访问则不必对公网开放）。
