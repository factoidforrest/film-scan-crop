#!/usr/bin/env python3
"""
Setup script for scan-crop package.
"""

from setuptools import setup, find_packages
import os

# Read the README file for long description
def read_readme():
    readme_path = os.path.join(os.path.dirname(__file__), 'README.md')
    if os.path.exists(readme_path):
        with open(readme_path, 'r', encoding='utf-8') as f:
            return f.read()
    return "Film negative/positive crop detection tool"

# Read version from __init__.py
def get_version():
    init_path = os.path.join(os.path.dirname(__file__), 'scan_crop', '__init__.py')
    if os.path.exists(init_path):
        with open(init_path, 'r', encoding='utf-8') as f:
            for line in f:
                if line.startswith('__version__'):
                    return line.split('=')[1].strip().strip('"\'')
    return "0.1.0"

setup(
    name="scan-crop",
    version=get_version(),
    author="Your Name",
    author_email="your.email@example.com",
    description="Film negative/positive crop detection tool with automatic border detection",
    long_description=read_readme(),
    long_description_content_type="text/markdown",
    url="https://github.com/yourusername/scan-crop",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: End Users/Desktop",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Multimedia :: Graphics :: Graphics Conversion",
        "Topic :: Scientific/Engineering :: Image Processing",
    ],
    python_requires=">=3.8",
    install_requires=[
        "opencv-python>=4.5.0",
        "numpy>=1.19.0",
    ],
    extras_require={
        "dev": [
            "pytest>=6.0",
            "black>=21.0",
            "flake8>=3.8",
            "mypy>=0.800",
        ],
        "build": [
            "pyinstaller>=5.0",
            "nuitka>=1.0",
        ],
    },
    entry_points={
        "console_scripts": [
            "scan-crop=scan_crop.cli:main",
        ],
    },
    include_package_data=True,
    zip_safe=False,
)
