# Scan-Crop

Film negative/positive crop detection tool with automatic border detection. Originally forked from this gist but heavily modified: https://gist.github.com/stecman/91cb5d28d330550a1dc56fa29215cb85

This is a beta. It's primarily designed to handle batch cropping and rotation of either positive or negative scans. I made it to run on an entire folder of lightroom exports after going through Negative Lab Pro, which odly doesn't yet have this feature. A golang port is in progress because python is horrid, but the python script currently works and can handle large folders ergonomically. It supports tiff and standard image formats, but not RAW, so you would typically use this after you export.

A binary in planned in the future to make distribution easier. For now, you have to install the python package as described below. 

## Features

- **Automatic polarity detection**: Detects whether input is a negative or positive scan
- **Smart border detection**: Finds film frame boundaries using OpenCV contour detection
- **Aspect ratio enforcement**: Optional 3:2 or 2:3 aspect ratio enforcement
- **Batch processing**: Process entire folders of images
- **Multiple output modes**: Overwrite in place, output to directory, or create cropped copies
- **Progress tracking**: Shows processing progress with counters

## Installation

### From source
```bash
git clone https://github.com/yourusername/scan-crop.git
cd scan-crop
pip install -e .
```

### From PyPI (when published, not yet)
```bash
pip install scan-crop
```

## Usage

### Basic usage
```bash
# Process a single image
scan-crop image.jpg

# Process with verbose output
scan-crop --verbose image.jpg

# Enforce 3:2 or 2:3 aspect ratio
scan-crop --enforce-32 image.jpg

# Process entire folder, output to subdirectory
scan-crop scans/ --output-dir cropped

# Process folder, overwrite originals
scan-crop scans/ --overwrite

# Dry run (show what would be done)
scan-crop --dry-run scans/
```

### Command line options

- `--verbose, -v`: Print debug information to stderr
- `--show, -s`: Display debug windows (requires GUI)
- `--enforce-32`: Enforce 3:2 (landscape) or 2:3 (portrait) aspect ratio
- `--dry-run`: Do not write output files, only show what would be done
- `--output-dir DIR`: When processing folders, write outputs to DIR
- `--overwrite`: When processing folders, overwrite original files

## Examples

```bash
# Basic crop detection
scan-crop negative.jpg

# With aspect ratio enforcement and verbose output
scan-crop --verbose --enforce-32 positive.jpg

# Process folder with progress tracking
scan-crop --verbose scans/ --output-dir cropped

# Overwrite originals in place
scan-crop scans/ --overwrite
```

## Output

The tool outputs 5 values to stdout:
1. Left crop (0.0-1.0)
2. Right crop (0.0-1.0) 
3. Top crop (0.0-1.0)
4. Bottom crop (0.0-1.0)
5. Rotation (degrees)

For batch processing, shows progress like:
```
[1/10] cropped image to 91% -> output/image1.jpg
[2/10] cropped image to 87% -> output/image2.jpg
```

## Requirements

- Python 3.8+
- OpenCV 4.5+
- NumPy 1.19+

## License

GPL License
