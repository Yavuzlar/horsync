@echo off
setlocal EnableExtensions
set "FRONTEND_PORT=3000"
set "BACKEND_PORT=3001"

if /I "%~1"=="--help" goto :help
if /I "%~1"=="-h" goto :help

set "ROOT_DIR=%~dp0"
if "%ROOT_DIR:~-1%"=="\" set "ROOT_DIR=%ROOT_DIR:~0,-1%"
set "FRONTEND_DIR=%ROOT_DIR%\frontend"
set "DATABASE_URL=postgres://horsync:horsync123@localhost:5433/horsync?sslmode=disable"

echo [1/8] Checking project workspace...
if not exist "%ROOT_DIR%\docker-compose.yml" (
  echo ERROR: docker-compose.yml not found in %ROOT_DIR%
  pause
  exit /b 1
)

echo [2/8] Checking prerequisite binaries in PATH...
where docker >nul 2>nul
if errorlevel 1 (
  echo ERROR: docker command not found. Please install Docker Desktop.
  pause
  exit /b 1
)
where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: go command not found. Please install Golang.
  pause
  exit /b 1
)
where npm >nul 2>nul
if errorlevel 1 (
  echo ERROR: npm command not found. Please install Node.js.
  pause
  exit /b 1
)

echo [3/8] Checking Docker Engine daemon status...
docker info >nul 2>nul
if errorlevel 1 (
  echo ERROR: Docker daemon is not running. Please launch Docker Desktop.
  pause
  exit /b 1
)

echo [4/8] Launching PostgreSQL database container...
pushd "%ROOT_DIR%"
docker compose up -d postgres
if errorlevel 1 (
  popd
  echo ERROR: Failed to start PostgreSQL container.
  pause
  exit /b 1
)
popd

echo [5/8] Checking frontend node_modules...
if not exist "%FRONTEND_DIR%\node_modules" (
  pushd "%FRONTEND_DIR%"
  echo Installing npm dependencies...
  call npm install
  if errorlevel 1 (
    popd
    echo ERROR: Failed to install npm packages.
    pause
    exit /b 1
  )
  popd
)

echo [6/8] Checking network port availability...
netstat -ano | findstr ":%FRONTEND_PORT%" | findstr "LISTENING" >nul
if not errorlevel 1 (
  echo ERROR: Frontend port %FRONTEND_PORT% is already occupied.
  pause
  exit /b 1
)
netstat -ano | findstr ":%BACKEND_PORT%" | findstr "LISTENING" >nul
if not errorlevel 1 (
  echo ERROR: Backend port %BACKEND_PORT% is already occupied.
  pause
  exit /b 1
)

echo [7/8] Compiling fresh unified Horsync executable...
if not exist "%ROOT_DIR%\bin" mkdir "%ROOT_DIR%\bin"
go build -o "%ROOT_DIR%\bin\horsync.exe" ./cmd/horsync
if errorlevel 1 (
  echo ERROR: Failed to compile Horsync executable binary from source.
  pause
  exit /b 1
)

echo [8/8] Booting backend and frontend in parallel...
start "Horsync Backend" cmd /k "chcp 65001 >nul && cd /d "%ROOT_DIR%" && set "DATABASE_URL=%DATABASE_URL%" && "%ROOT_DIR%\bin\horsync.exe" --server"
start "Horsync Frontend" cmd /k "chcp 65001 >nul && cd /d "%FRONTEND_DIR%" && npm run dev"

echo Opening browser at http://localhost:%FRONTEND_PORT%...
timeout /t 4 /nobreak >nul
start "" "http://localhost:%FRONTEND_PORT%"

echo.
echo =========================================
echo HORSYNC STARTED SUCCESSFULLY!
echo Login:
echo   admin@horsync.local
echo   admin12345
echo =========================================
exit /b 0

:help
echo Usage: run_mvp.bat
pause
exit /b 0
