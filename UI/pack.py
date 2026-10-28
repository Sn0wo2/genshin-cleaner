import shutil
import subprocess
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

ARCH = sys.argv[1] if len(sys.argv) > 1 else "x64"
assert ARCH in ("x64", "ARM64"), f"bad arch: {ARCH}"

ROOT = Path(__file__).resolve().parent.parent
VERSION = ET.parse(ROOT / "installer.wxs").find(
    "{http://wixtoolset.org/schemas/v4/wxs}Package"
).attrib["Version"]
PUBLISH = ROOT / "dist" / "publish"
MSI = ROOT / "dist" / "Installer" / f"genshin-cleaner-{VERSION}-{ARCH}.msi"

shutil.rmtree(ROOT / "dist", ignore_errors=True)
subprocess.run([
    "dotnet", "publish", str(ROOT / "UI" / "genshin-cleanner-ui.csproj"),
    "-nologo", "-c", "Release",
    f"-p:Version={VERSION}", f"-p:Platform={ARCH}", f"-p:RuntimeIdentifier=win-{ARCH.lower()}",
    "--self-contained", "true", f"-p:PublishDir={PUBLISH}\\",
], check=True)

if not (PUBLISH / "genshin-cleaner.pri").is_file():
    raise RuntimeError("Published WinUI resources are missing: genshin-cleaner.pri")

MSI.parent.mkdir(parents=True, exist_ok=True)
subprocess.run([
    "wix", "build", "-nologo", "-arch", ARCH.lower(), "-culture", "zh-CN",
    "-ext", "WixToolset.UI.wixext/7.0.0",
    "-d", f"PUBLISH={PUBLISH}", "-o", str(MSI), str(ROOT / "installer.wxs"),
], check=True, cwd=ROOT)
print(MSI)
