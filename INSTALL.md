# ctlptl Installation Appendix

## Recommended

### Homebrew (Mac/Linux)

```
brew install tilt-dev/tap/ctlptl
```

### Scoop (Windows)

```
scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket
scoop install ctlptl
```

## Alternative

### Point and click

Visit [the releases page](https://github.com/tilt-dev/ctlptl/releases) and
download the pre-build binaries for your architecture.

### Command-line

On macOS:

```bash
export CTLPTL_VERSION="0.4.2"
curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v$CTLPTL_VERSION/ctlptl.$CTLPTL_VERSION.mac.x86_64.tar.gz | tar -xzv ctlptl && \
  sudo mv ctlptl /usr/local/bin/ctlptl
```

On Linux:

```bash
export CTLPTL_VERSION="0.4.2"
curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v$CTLPTL_VERSION/ctlptl.$CTLPTL_VERSION.linux.x86_64.tar.gz | tar -xzv ctlptl && \
  sudo mv ctlptl /usr/local/bin/ctlptl
```

On Windows:

```powershell
$CTLPTL_VERSION = "0.4.2"
Invoke-WebRequest "https://github.com/tilt-dev/ctlptl/releases/download/v$CTLPTL_VERSION/ctlptl.$CTLPTL_VERSION.windows.x86_64.zip" -OutFile "ctlptl.zip"
Expand-Archive "ctlptl.zip" -DestinationPath "ctlptl"
Move-Item -Force -Path "ctlptl\ctlptl.exe" -Destination "$home\bin\ctlptl.exe"
```
