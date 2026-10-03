@echo off
setlocal

echo ============================================
echo GhostChat - Windows Build
echo ============================================
echo.

if "%YOUTUBE_API_KEY%"=="" (
    echo YOUTUBE_API_KEY is not set.
    echo The build will still work, but YouTube will use
    echo its public/Innertube fallback instead of streamList.
    echo.
    echo For a release build, set YOUTUBE_API_KEY before
    echo running this script.
    echo.
) else (
    echo YouTube API key detected. It will be embedded
    echo into this build and will NOT be written to source.
    echo.
)

where wails3 >nul 2>&1
if errorlevel 1 (
    echo ERROR: wails3 was not found on PATH.
    echo Install Wails 3 first, then run this script again.
    exit /b 1
)

wails3 task windows:build

if errorlevel 1 (
    echo.
    echo BUILD FAILED.
    exit /b 1
)

echo.
echo BUILD COMPLETE:
echo   bin\ghost-chat.exe
exit /b 0
