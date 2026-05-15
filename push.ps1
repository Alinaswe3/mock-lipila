$IMAGE = "ghcr.io/alinaswe3/mock-lipila"
$TIMESTAMP = Get-Date -Format "yyyyMMdd-HHmmss"

Write-Host "Building production image..." -ForegroundColor Cyan
docker build -t "${IMAGE}:${TIMESTAMP}" -t "${IMAGE}:latest" .

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host "Pushing ${IMAGE}:${TIMESTAMP}..." -ForegroundColor Cyan
docker push "${IMAGE}:${TIMESTAMP}"
docker push "${IMAGE}:latest"

if ($LASTEXITCODE -ne 0) {
    Write-Host "Push failed! Run: docker login ghcr.io -u alinaswe3" -ForegroundColor Yellow
    exit 1
}

Write-Host "Done! Pushed ${IMAGE}:${TIMESTAMP} (also tagged as latest)" -ForegroundColor Green
