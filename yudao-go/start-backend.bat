@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

REM ============================================================
REM  yudao-go 一键启动后端
REM    1) Docker 依赖容器(MySQL 13306 / Redis 16381 / Jaeger 16686)
REM    2) Go 后端 server.exe (48090) -- 自动 go build
REM    3) BPM Java 服务 yudao-server.jar (48081) -- 工作流模块
REM  前端 vite 单独跑: cd ..\yudao-ui-admin-vue3 ^&^& pnpm run dev
REM ============================================================

REM 切到脚本所在目录(yudao-go 根)
cd /d "%~dp0"

set "GO_DIR=%~dp0"
set "ROOT=%~dp0.."
set "BPM_DIR=%ROOT%\ruoyi-vue-pro"
set "OTEL_AGENT=%ROOT%\tools\opentelemetry-javaagent.jar"
set "BPM_JAR=%BPM_DIR%\yudao-server\target\yudao-server.jar"

echo.
echo === [1/3] Docker dependencies ===
docker compose -f "%GO_DIR%deploy\docker-compose.yml" up -d
if errorlevel 1 (
    echo [ERROR] docker compose failed - is Docker Desktop running?
    pause
    exit /b 1
)

echo.
echo === [2/3] Go backend: build + restart on :48090 ===
taskkill /F /IM server.exe >nul 2>&1
go build -o bin\server.exe .\cmd\server
if errorlevel 1 (
    echo [ERROR] go build failed
    pause
    exit /b 1
)
start "yudao-go :48090" cmd /k "bin\server.exe"

echo.
echo === [3/3] BPM Java service on :48081 ===
if not exist "%BPM_JAR%" (
    echo [WARN] BPM jar not found at:
    echo        %BPM_JAR%
    echo        run: cd %BPM_DIR% ^&^& mvn clean package -pl yudao-server -am -DskipTests
    echo        skipping BPM startup.
) else (
    if not exist "%OTEL_AGENT%" (
        echo [WARN] OTel agent not found at %OTEL_AGENT% - starting BPM without tracing
        start "yudao-bpm :48081" cmd /k "cd /d %BPM_DIR% && java -jar yudao-server\target\yudao-server.jar"
    ) else (
        start "yudao-bpm :48081" cmd /k "cd /d %BPM_DIR% && java -javaagent:%OTEL_AGENT% -Dotel.service.name=yudao-server-bpm -Dotel.exporter.otlp.endpoint=http://127.0.0.1:14318 -Dotel.exporter.otlp.protocol=http/protobuf -Dotel.metrics.exporter=none -Dotel.logs.exporter=none -jar yudao-server\target\yudao-server.jar"
    )
)

echo.
echo ============================================================
echo   started -- check the two new terminal windows for logs
echo   Go backend : http://localhost:48090/health
echo   BPM service: http://localhost:48081  (30-60s warm-up)
echo   Frontend   : cd %ROOT%\yudao-ui-admin-vue3 ^&^& pnpm run dev   (port 5174)
echo ============================================================
endlocal
