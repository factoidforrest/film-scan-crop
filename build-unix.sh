#!/bin/bash
echo "Building scan-crop for Unix/Linux/macOS..."

# Install build dependencies
pip install pyinstaller nuitka

# Build with PyInstaller
python build.py pyinstaller

# Build with Nuitka
python build.py nuitka

echo "Build complete! Check dist/ and scan-crop.dist/ folders."
