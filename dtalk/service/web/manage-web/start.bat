@echo off
echo 启动群组管理系统Web界面...
echo.

REM 检查Python是否安装
python --version >nul 2>&1
if %errorlevel% equ 0 (
    echo 使用Python启动本地服务器...
    echo 访问地址: http://localhost:8000
    echo 按 Ctrl+C 停止服务器
    echo.
    python -m http.server 8000
) else (
    echo Python未安装，请直接在浏览器中打开 index.html 文件
    echo.
    pause
)

