package manageweb

import "embed"

// Files 是运营页面。chatadmin 镜像只带二进制，页面打进二进制后由 /console/ 提供。
//
//go:embed index.html config.js update-config.js css js
var Files embed.FS
