function Zip-Release {
    param (
    [string]$sourceDir,
    [string]$destFile
    )


    # While we could use zipfile from system.io.compression.filesystem, this
    # uses Windows-style path separators all the time! Fixed in CLR 4.6.1 but
    # default Powershell can't access this, PITA. Let's just use the 7z command

    $tempdest =  [System.IO.Path]::GetTempPath() + [System.IO.Path]::GetRandomFileName() + ".zip"
    $zipargs = "a `"$tempdest`" $sourcedir\*"

    try {
        7z a "$tempdest" "$sourcedir\*" | Out-Null
    }
    catch {
        Remove-Item $tempdest -Force
        throw
    }

    # If successful, move into final location
    if (Test-Path $destFile) {
        Remove-item $destFile
    }
    Move-Item $tempdest $destFile
}

$package = "github.com/sinbad/lfs-folderstore"
$archivename = "lfs-folderstore"

# Check dirty repo
git diff --no-patch --exit-code
if ($LASTEXITCODE -ne 0) {
    Write-Output "Working copy is not clean (unstaged changes)"
    Exit $LASTEXITCODE
}
git diff --no-patch --cached --exit-code
if ($LASTEXITCODE -ne 0) {
    Write-Output "Working copy is not clean (staged changes)"
    Exit $LASTEXITCODE
}

# Check that the latest tag is present directly on HEAD
$Version = (git describe --exact-match | Out-String).Trim()

if ($LASTEXITCODE -ne 0) {
    Write-Output "No version tag on HEAD"
    Exit $LASTEXITCODE
}

Write-Output "Building version: $Version"

$BuildConfigs = @{
    "windows" = @("amd64", "386");
    "linux" = @("amd64", "386");
    "darwin" = @("amd64");
}

foreach ($BuildOS in $BuildConfigs.GetEnumerator()) {
    foreach ($Arch in $BuildOS.Value) {
        Write-Output "- $($BuildOS.Name):$Arch"

        $outputdir = "$archivename-$($BuildOS.Name)-$Arch"
        mkdir -Force $outputdir | Out-Null
        Push-Location $outputdir

        $env:GOOS=$($BuildOS.Name)
        $env:GOARCH=$Arch 
        go build -ldflags "-X $package/cmd.Version=$Version" $package

        Pop-Location

        $zipname = "$outputdir-$Version.zip"
        Zip-Release $outputdir $zipname
        Remove-Item -Force -Recurse $outputdir

        Write-Output "  Done: $zipname"
    }
}
