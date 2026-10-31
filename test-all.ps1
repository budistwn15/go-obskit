$ErrorActionPreference = "Stop"

$modules = @(
    ".",
    "adapters/nethttp",
    "adapters/ginx",
    "adapters/fiberx",
    "adapters/gormx",
    "adapters/otel",
    "examples"
)

foreach ($module in $modules) {
    Push-Location $module
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}
