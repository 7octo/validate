@echo off
:: Database Migration Tool for Windows
:: Usage: migrate.bat [command] [options]

:: Set default values
SET MIGRATION_DIR=db\migrations
SET DB_DRIVER=postgres
SET DB_HOST=localhost
SET DB_PORT=5432
SET DB_NAME=mydatabase
SET DB_USER=postgres
SET DB_PASS=postgres
SET DB_SSL_MODE=disable

:: Load environment variables from .env file if it exists
if exist .env (
    for /f "tokens=1,2 delims==" %%A in (.env) do (
        set %%A=%%B
    )
)

:: Build connection string
SET DB_URL=%DB_DRIVER%://%DB_USER%:%DB_PASS%@%DB_HOST%:%DB_PORT%/%DB_NAME%?sslmode=%DB_SSL_MODE%

:: Check if migrate command exists
where migrate >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo Error: migrate command not found in PATH
    echo Install golang-migrate first: https://github.com/golang-migrate/migrate
    exit /b 1
)

:: Main command switch
if "%1"=="" goto help

if "%1"=="up" (
    echo Running migrations...
    migrate -path %MIGRATION_DIR% -database "%DB_URL%" up
    if %ERRORLEVEL% neq 0 (
        echo Migration failed
        exit /b 1
    )
    echo Migrations completed successfully
    goto :eof
)

if "%1"=="down" (
    echo Reverting last migration...
    migrate -path %MIGRATION_DIR% -database "%DB_URL%" down 1
    if %ERRORLEVEL% neq 0 (
        echo Migration revert failed
        exit /b 1
    )
    echo Migration reverted successfully
    goto :eof
)

if "%1"=="create" (
    if "%2"=="" (
        set /p name="Enter migration name: "
    ) else (
        set name=%2
    )
    echo Creating new migration: %name%
    migrate create -ext sql -dir %MIGRATION_DIR% -seq %name%
    goto :eof
)

if "%1"=="force" (
    if "%2"=="" (
        echo Error: version number required
        goto help
    )
    echo Forcing migration version %2...
    migrate -path %MIGRATION_DIR% -database "%DB_URL%" force %2
    goto :eof
)

if "%1"=="version" (
    migrate -path %MIGRATION_DIR% -database "%DB_URL%" version
    goto :eof
)

:help
echo Usage: migrate.bat [command]
echo Commands:
echo   up       - Run all available migrations
echo   down     - Roll back the last migration
echo   create   - Create a new migration file
echo   force V  - Set migration version to V
echo   version  - Show current migration version
echo 
echo Configuration:
echo   Set variables in .env file or edit this script
echo   Current settings:
echo     MIGRATION_DIR=%MIGRATION_DIR%
echo     DB_URL=%DB_URL%
exit /b 0
