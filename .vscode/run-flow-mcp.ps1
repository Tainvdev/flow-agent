$ErrorActionPreference = 'Stop'

$tokenPath = Join-Path $env:TEMP 'flow-agent-service.token'
if (-not (Test-Path -LiteralPath $tokenPath -PathType Leaf)) {
    [Console]::Error.WriteLine('Flow Agent MCP: service token file is missing.')
    exit 1
}

$token = (Get-Content -Raw -LiteralPath $tokenPath).Trim()
if ([string]::IsNullOrWhiteSpace($token)) {
    [Console]::Error.WriteLine('Flow Agent MCP: service token file is empty.')
    exit 1
}

$workspaceRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$apiRoot = Join-Path $workspaceRoot 'flow-agent'
$env:FLOW_ENGINE_MODE = 'service'
$env:FLOW_SERVICE_URL = 'http://127.0.0.1:9223'
$env:FLOW_INTERNAL_TOKEN = $token
$env:FLOW_OUTPUT_DIR = Join-Path $apiRoot 'output'

Set-Location -LiteralPath $apiRoot
& python main.py mcp
exit $LASTEXITCODE
