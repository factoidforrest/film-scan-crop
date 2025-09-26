# Makefile for scan-crop

.PHONY: install dev-install build clean test lint format help

# Default target
help:
	@echo "Scan-Crop Makefile"
	@echo "=================="
	@echo ""
	@echo "Available targets:"
	@echo "  install      Install package in development mode"
	@echo "  dev-install  Install with development dependencies"
	@echo "  build        Build package for distribution"
	@echo "  build-exe    Build executable with PyInstaller"
	@echo "  build-nuitka Build executable with Nuitka"
	@echo "  test         Run tests"
	@echo "  lint         Run linting checks"
	@echo "  format       Format code with black"
	@echo "  clean        Clean build artifacts"
	@echo "  help         Show this help message"

# Install in development mode
install:
	pip install -e .

# Install with development dependencies
dev-install:
	pip install -e ".[dev]"

# Build package
build:
	python -m build

# Build executable with PyInstaller
build-exe:
	pip install pyinstaller
	python build.py pyinstaller

# Build executable with Nuitka
build-nuitka:
	pip install nuitka
	python build.py nuitka

# Run tests
test:
	pytest

# Run linting
lint:
	flake8 scan_crop/
	mypy scan_crop/

# Format code
format:
	black scan_crop/

# Clean build artifacts
clean:
	rm -rf build/
	rm -rf dist/
	rm -rf *.egg-info/
	rm -rf scan-crop.dist/
	rm -rf scan-crop.build/
	find . -type d -name __pycache__ -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

# Create build scripts
scripts:
	python build.py scripts
