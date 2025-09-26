@echo off
echo Building scan-crop for Windows...

REM Install build dependencies
pip install pyinstaller nuitka

REM Build with PyInstaller
python build.py pyinstaller

REM Build with Nuitka  
python build.py nuitka

echo Build complete! Check dist/ and scan-crop.dist/ folders.
pause
