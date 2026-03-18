$env:GOOS = "linux"
$env:GOARCH = "amd64"

Write-Host "Building cd-agent for Linux (amd64)..."
go build -o cd-agent-linux main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! The binary is 'cd-agent-linux'." -ForegroundColor Green
} else {
    Write-Host "Build failed." -ForegroundColor Red
}

# Reset environment variables (optional, but good practice)
$env:GOOS = ""
$env:GOARCH = ""
