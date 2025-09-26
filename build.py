#!/usr/bin/env python3
"""
Build script for creating portable executables with PyInstaller and Nuitka.
"""

import os
import sys
import subprocess
import shutil
from pathlib import Path

def run_command(cmd, description):
    """Run a command and handle errors."""
    print(f"\n{description}...")
    print(f"Running: {' '.join(cmd)}")
    
    try:
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        print("✓ Success")
        return True
    except subprocess.CalledProcessError as e:
        print(f"✗ Failed: {e}")
        print(f"stdout: {e.stdout}")
        print(f"stderr: {e.stderr}")
        return False

def build_with_pyinstaller():
    """Build executable with PyInstaller."""
    print("\n" + "="*50)
    print("Building with PyInstaller")
    print("="*50)
    
    # Clean previous builds
    if os.path.exists("dist"):
        shutil.rmtree("dist")
    if os.path.exists("build"):
        shutil.rmtree("build")
    
    # PyInstaller command
    cmd = [
        "pyinstaller",
        "--onefile",  # Single executable file
        "--name", "scan-crop",
        "--add-data", "scan_crop:scan_crop",  # Include package data
        "--hidden-import", "cv2",
        "--hidden-import", "numpy",
        "--console",  # Keep console for CLI
        "scan_crop/cli.py"
    ]
    
    return run_command(cmd, "Building PyInstaller executable")

def build_with_nuitka():
    """Build executable with Nuitka."""
    print("\n" + "="*50)
    print("Building with Nuitka")
    print("="*50)
    
    # Clean previous builds
    if os.path.exists("scan-crop.dist"):
        shutil.rmtree("scan-crop.dist")
    if os.path.exists("scan-crop.build"):
        shutil.rmtree("scan-crop.build")
    
    # Nuitka command
    cmd = [
        "nuitka",
        "--onefile",  # Single executable file
        "--output-filename=scan-crop",
        "--include-package=scan_crop",
        "--include-module=cv2",
        "--include-module=numpy",
        "--assume-yes-for-downloads",
        "scan_crop/cli.py"
    ]
    
    return run_command(cmd, "Building Nuitka executable")

def create_build_script():
    """Create platform-specific build scripts."""
    
    # Windows batch script
    windows_script = """@echo off
echo Building scan-crop for Windows...

REM Install build dependencies
pip install pyinstaller nuitka

REM Build with PyInstaller
python build.py pyinstaller

REM Build with Nuitka  
python build.py nuitka

echo Build complete! Check dist/ and scan-crop.dist/ folders.
pause
"""
    
    with open("build-windows.bat", "w") as f:
        f.write(windows_script)
    
    # Unix shell script
    unix_script = """#!/bin/bash
echo "Building scan-crop for Unix/Linux/macOS..."

# Install build dependencies
pip install pyinstaller nuitka

# Build with PyInstaller
python build.py pyinstaller

# Build with Nuitka
python build.py nuitka

echo "Build complete! Check dist/ and scan-crop.dist/ folders."
"""
    
    with open("build-unix.sh", "w") as f:
        f.write(unix_script)
    
    # Make executable
    os.chmod("build-unix.sh", 0o755)
    
    print("Created build-windows.bat and build-unix.sh")

def main():
    """Main build function."""
    if len(sys.argv) > 1:
        target = sys.argv[1].lower()
        
        if target == "pyinstaller":
            success = build_with_pyinstaller()
        elif target == "nuitka":
            success = build_with_nuitka()
        elif target == "scripts":
            create_build_script()
            return
        else:
            print(f"Unknown target: {target}")
            print("Usage: python build.py [pyinstaller|nuitka|scripts]")
            return
        
        if success:
            print(f"\n✓ {target.title()} build completed successfully!")
        else:
            print(f"\n✗ {target.title()} build failed!")
            sys.exit(1)
    else:
        print("Scan-Crop Build Script")
        print("="*50)
        print("This script builds portable executables for scan-crop.")
        print("\nUsage:")
        print("  python build.py pyinstaller  # Build with PyInstaller")
        print("  python build.py nuitka       # Build with Nuitka")
        print("  python build.py scripts      # Create platform build scripts")
        print("\nPrerequisites:")
        print("  pip install pyinstaller nuitka")
        print("\nNote: Nuitka typically produces faster executables but")
        print("PyInstaller is more reliable for complex dependencies like OpenCV.")

if __name__ == "__main__":
    main()
