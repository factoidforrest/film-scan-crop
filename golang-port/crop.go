package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"flag"
	
	"gocv.io/x/gocv"
)

// Detection settings
const (
	MaxCoverage   = 0.98
	InsetPercent  = 0.005
)

var verbose bool

// Point2f represents a 2D point with float coordinates
type Point2f struct {
	X, Y float32
}

// RotatedRect represents a rotated rectangle
type RotatedRect struct {
	Center Point2f
	Size   Point2f  
	Angle  float64
}

func main() {
	var showWindows bool
	var enforce32 bool
	var dryRun bool
	var outputDir string
	var overwrite bool
	
	flag.BoolVar(&verbose, "verbose", false, "Print debug information")
	flag.BoolVar(&showWindows, "show", false, "Display debug windows")
	flag.BoolVar(&enforce32, "enforce-32", false, "Enforce 3:2 or 2:3 aspect ratio")
	flag.BoolVar(&dryRun, "dry-run", false, "Do not write cropped output image")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for processed images")
	flag.BoolVar(&overwrite, "overwrite", false, "Overwrite original images")
	
	flag.Parse()
	
	files := flag.Args()
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] image_files...\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	
	// Expand directories
	var inputFiles []string
	for _, file := range files {
		if isDir(file) {
			if !overwrite && outputDir == "" {
				fmt.Fprintf(os.Stderr, "ERROR: When passing a folder, provide --output-dir or --overwrite\n")
				os.Exit(2)
			}
			dirFiles, err := expandDirectory(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to list directory '%s': %v\n", file, err)
				continue
			}
			inputFiles = append(inputFiles, dirFiles...)
		} else {
			inputFiles = append(inputFiles, file)
		}
	}
	
	total := len(inputFiles)
	
	for idx, filename := range inputFiles {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[%d/%d] WARNING: Skipping '%s': %v\n", idx+1, total, filename, r)
				}
			}()
			
			img, left, right, top, bottom, intermediates := processImage(filename, showWindows, enforce32)
			defer img.Close()
			
			// Write cropped output unless dry-run
			var outPath string
			if !dryRun && !img.Empty() {
				h, w := img.Rows(), img.Cols()
				x0 := int(math.Max(0, math.Min(float64(w-1), left*float64(w))))
				x1 := int(math.Max(0, math.Min(float64(w), right*float64(w))))
				y0 := int(math.Max(0, math.Min(float64(h-1), top*float64(h))))
				y1 := int(math.Max(0, math.Min(float64(h), bottom*float64(h))))
				
				if verbose {
					fmt.Fprintf(os.Stderr, "crop px (x0,x1,y0,y1)= %d %d %d %d\n", x0, x1, y0, y1)
				}
				
				if x1 > x0 && y1 > y0 {
					rect := image.Rect(x0, y0, x1, y1)
					cropped := img.Region(rect)
					defer cropped.Close()
					
					// Determine output path
					if overwrite {
						outPath = filename
					} else if outputDir != "" {
						baseDir := filepath.Dir(filename)
						var finalOutputDir string
						if filepath.IsAbs(outputDir) {
							finalOutputDir = outputDir
						} else {
							finalOutputDir = filepath.Join(filepath.Dir(baseDir), outputDir)
						}
						os.MkdirAll(finalOutputDir, 0755)
						outPath = filepath.Join(finalOutputDir, filepath.Base(filename))
					} else {
						ext := filepath.Ext(filename)
						base := strings.TrimSuffix(filename, ext)
						outPath = base + "_cropped" + ext
					}
					
					ok := gocv.IMWrite(outPath, cropped)
					if verbose {
						fmt.Fprintf(os.Stderr, "wrote cropped: %v %s\n", ok, outPath)
					}
				}
			}
			
			// Cleanup intermediates
			for _, p := range intermediates {
				os.Remove(p)
				if verbose {
					fmt.Fprintf(os.Stderr, "cleaned up intermediate: %s\n", p)
				}
			}
			
			// Progress output
			retained := math.Max(0.0, (right-left)*(bottom-top))
			pct := int(math.Round(retained * 100))
			status := fmt.Sprintf("[%d/%d] ", idx+1, total)
			
			var line string
			if dryRun {
				line = fmt.Sprintf("%swould crop to %d%% (%s)", status, pct, filepath.Base(filename))
			} else {
				dest := outPath
				if dest == "" {
					dest = "(no output)"
				}
				line = fmt.Sprintf("%scropped image to %d%% -> %s", status, pct, dest)
			}
			fmt.Println(line)
		}()
	}
}

func processImage(filename string, showWindows, enforce32 bool) (gocv.Mat, float64, float64, float64, float64, []string) {
	if !fileExists(filename) {
		panic(fmt.Sprintf("Could not find file '%s'", filename))
	}
	
	var intermediates []string
	
	// Read image
	img := gocv.IMRead(filename, gocv.IMReadColor)
	if img.Empty() {
		panic("failed to read image")
	}
	
	if verbose {
		fmt.Fprintf(os.Stderr, "file= %s\n", filename)
		fmt.Fprintf(os.Stderr, "image.shape= %dx%dx%d dtype= %v\n", img.Rows(), img.Cols(), img.Channels(), img.Type())
	}
	
	rawRect := findExposureBounds(img, showWindows)
	if verbose {
		fmt.Fprintf(os.Stderr, "rawRect= %+v\n", rawRect)
	}
	
	// Default outputs
	cropLeft := 0.0
	cropRight := 1.0
	cropTop := 0.0
	cropBottom := 1.0
	rotation := 0.0
	
	if rawRect != nil {
		// Average height and width to get constant inset
		insetPixels := ((rawRect.Size.X + rawRect.Size.Y) / 2.0) * InsetPercent
		
		insetRect := &RotatedRect{
			Center: rawRect.Center,
			Size:   Point2f{X: rawRect.Size.X - insetPixels, Y: rawRect.Size.Y - insetPixels},
			Angle:  rawRect.Angle,
		}
		
		rect, aspectChanged := correctAspectRatio(insetRect, 1.5, 0.3)
		if verbose {
			fmt.Fprintf(os.Stderr, "insetRect= %+v rectCorrected= %+v aspectChanged= %v\n", insetRect, rect, aspectChanged)
		}
		
		cropLeft, cropRight, cropTop, cropBottom = calculateCropCoordinates(rect, img.Rows(), img.Cols())
		
		// Enforce 3:2 aspect ratio if requested
		if enforce32 {
			cropLeft, cropRight, cropTop, cropBottom = enforce32AspectRatio(
				cropLeft, cropRight, cropTop, cropBottom, img.Cols(), img.Rows())
		}
		
		// Final 1% inward crop preserving aspect ratio
		prev := [4]float64{cropLeft, cropRight, cropTop, cropBottom}
		cropLeft, cropRight, cropTop, cropBottom = shrinkCropUniform(
			cropLeft, cropRight, cropTop, cropBottom, 0.01)
		if verbose {
			fmt.Fprintf(os.Stderr, "final 1%% shrink from %v to %v\n", prev, [4]float64{cropLeft, cropRight, cropTop, cropBottom})
		}
		
		// Rotation for Lightroom
		rotation = -rect.Angle
		if rotation > 45 {
			rotation -= 90
		} else if rotation < -90 {
			rotation += 45
		}
		
		if verbose {
			fmt.Fprintf(os.Stderr, "rotation= %f\n", rotation)
			fmt.Fprintf(os.Stderr, "crops LRTB= %f %f %f %f\n", cropLeft, cropRight, cropTop, cropBottom)
		}
		
		// Draw debug overlays
		debugImg := img.Clone()
		drawDebugOverlays(debugImg, rawRect, insetRect, rect)
		
		// Write results
		cropData := []float64{cropLeft, cropRight, cropTop, cropBottom, rotation}
		for _, v := range cropData {
			fmt.Println(v)
		}
		
		txtPath := filename + ".txt"
		writeCropData(txtPath, cropData)
		intermediates = append(intermediates, txtPath)
		
		analysisPath := filename + "-analysis.jpg"
		gocv.IMWrite(analysisPath, debugImg)
		intermediates = append(intermediates, analysisPath)
		debugImg.Close()
		
		if showWindows {
			window := gocv.NewWindow("image")
			defer window.Close()
			
			resized := gocv.NewMat()
			defer resized.Close()
			gocv.Resize(debugImg, &resized, image.Point{}, 0.75, 0.75, gocv.InterpolationLinear)
			
			window.IMShow(resized)
			window.WaitKey(0)
		}
	} else {
		// Even when no rect found, still emit default crop data
		cropData := []float64{cropLeft, cropRight, cropTop, cropBottom, rotation}
		for _, v := range cropData {
			fmt.Println(v)
		}
		
		txtPath := filename + ".txt"
		writeCropData(txtPath, cropData)
		intermediates = append(intermediates, txtPath)
	}
	
	return img, cropLeft, cropRight, cropTop, cropBottom, intermediates
}

func findExposureBounds(img gocv.Mat, showOutputWindow bool) *RotatedRect {
	// Detect polarity and optionally invert for processing
	polarity := detectScanPolarity(img)
	workImg := img.Clone()
	defer workImg.Close()
	
	if polarity == "positive" {
		// Invert positive to negative-like for processing
		gocv.BitwiseNot(workImg, &workImg)
		if verbose {
			fmt.Fprintf(os.Stderr, "inverted positive image for processing\n")
		}
	}
	
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(workImg, &gray, gocv.ColorBGRToGray)
	
	// Smooth out noise and maximize brightness range
	bilateralFiltered := gocv.NewMat()
	defer bilateralFiltered.Close()
	gocv.BilateralFilter(gray, &bilateralFiltered, 11, 17, 17)
	
	equalized := gocv.NewMat()
	defer equalized.Close()
	gocv.EqualizeHist(bilateralFiltered, &equalized)
	
	ignoreMask := createIgnoreMask(workImg, equalized, polarity)
	defer ignoreMask.Close()
	
	// Get min/max region of interest areas
	height, width := workImg.Rows(), workImg.Cols()
	maxArea := (float64(height) * MaxCoverage) * (float64(width) * MaxCoverage)
	minCaptureArea := maxArea * 0.65
	
	var results []*RotatedRect
	var bestRect *RotatedRect
	bestArea := 0.0
	
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Point{5, 5})
	defer kernel.Close()
	
	for lowerThreshold := 0; lowerThreshold < 240; lowerThreshold += 5 {
		// Use negative logic (THRESH_BINARY_INV) since we invert positives
		binary := gocv.NewMat()
		gocv.Threshold(equalized, &binary, float32(lowerThreshold), 255, gocv.ThresholdBinaryInv)
		
		masked := gocv.NewMat()
		gocv.BitwiseAnd(ignoreMask, binary, &masked)
		binary.Close()
		
		// Morphology
		dilated := gocv.NewMat()
		gocv.Dilate(masked, &dilated, kernel)
		masked.Close()
		
		eroded := gocv.NewMat()
		gocv.Erode(dilated, &eroded, kernel)
		dilated.Close()
		
		rect, area := findLargestContourRect(eroded)
		
		if verbose {
			fmt.Fprintf(os.Stderr, "threshold= %d area= %f rect= %+v\n", lowerThreshold, area, rect)
		}
		
		// Track best seen rect by area
		if rect != nil && area > bestArea {
			bestArea = area
			bestRect = rect
		}
		
		// Stop once a valid result is returned
		if rect != nil && area >= maxArea {
			eroded.Close()
			break
		}
		
		if rect != nil && area >= minCaptureArea {
			results = append(results, rect)
		}
		
		if showOutputWindow {
			debugImg := gocv.NewMat()
			gocv.CvtColor(eroded, &debugImg, gocv.ColorGrayToBGR)
			
			if rect != nil {
				drawRotatedRect(debugImg, rect, image.RGBA{0, 255, 0, 255}, 3) // Green for collected
			}
			
			// Draw threshold text
			gocv.PutText(&debugImg, fmt.Sprintf("Threshold: %d", lowerThreshold), 
				image.Point{20, 30}, gocv.FontHersheyPlain, 2, 
				image.RGBA{0, 150, 255, 255}, 2)
			
			window := gocv.NewWindow("image")
			resized := gocv.NewMat()
			gocv.Resize(debugImg, &resized, image.Point{}, 0.75, 0.75, gocv.InterpolationLinear)
			window.IMShow(resized)
			window.WaitKey(1)
			window.Close()
			resized.Close()
			debugImg.Close()
		}
		
		eroded.Close()
	}
	
	// Prefer median of good results; fall back to best seen rect
	median := medianRect(results)
	if median != nil {
		return median
	}
	return bestRect
}

func detectScanPolarity(img gocv.Mat) string {
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)
	
	h, w := gray.Rows(), gray.Cols()
	band := int(math.Max(5, float64(min(h, w))*0.02))
	
	// Sample border bands
	topBand := gray.Region(image.Rect(0, 0, w, band))
	bottomBand := gray.Region(image.Rect(0, h-band, w, h))
	leftBand := gray.Region(image.Rect(0, 0, band, h))
	rightBand := gray.Region(image.Rect(w-band, 0, w, h))
	
	meanVal := (gocv.Mean(topBand).Val1 + gocv.Mean(bottomBand).Val1 + 
		gocv.Mean(leftBand).Val1 + gocv.Mean(rightBand).Val1) / 4.0
	
	topBand.Close()
	bottomBand.Close()
	leftBand.Close()
	rightBand.Close()
	
	var polarity string
	if meanVal >= 150.0 {
		polarity = "negative"
	} else {
		polarity = "positive"
	}
	
	if verbose {
		fmt.Fprintf(os.Stderr, "polarity mean border gray= %f => %s\n", meanVal, polarity)
	}
	
	return polarity
}

func createIgnoreMask(img, gray gocv.Mat, polarity string) gocv.Mat {
	// Mask brightest spots
	ignoreMask := gocv.NewMat()
	gocv.Threshold(gray, &ignoreMask, 240, 255, gocv.ThresholdBinary)
	
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Point{3, 3})
	defer kernel.Close()
	
	dilated := gocv.NewMat()
	defer dilated.Close()
	gocv.Dilate(ignoreMask, &dilated, kernel)
	ignoreMask.Close()
	
	if polarity == "negative" {
		// Ignore areas of low saturation (common in negative scans)
		hsv := gocv.NewMat()
		defer hsv.Close()
		gocv.CvtColor(img, &hsv, gocv.ColorBGRToHSV)
		
		blurred := gocv.NewMat()
		defer blurred.Close()
		gocv.GaussianBlur(hsv, &blurred, image.Point{5, 5}, 0, 0, gocv.BorderDefault)
		
		satMask := gocv.NewMat()
		defer satMask.Close()
		lower := gocv.NewScalar(0, 0, 0, 0)
		upper := gocv.NewScalar(255, 7, 255, 0)
		gocv.InRangeWithScalar(blurred, lower, upper, &satMask)
		
		combined := gocv.NewMat()
		gocv.BitwiseOr(dilated, satMask, &combined)
		dilated.Close()
		
		// Flip to create keep mask
		final := gocv.NewMat()
		gocv.BitwiseNot(combined, &final)
		combined.Close()
		return final
	}
	
	// Flip to create keep mask
	final := gocv.NewMat()
	gocv.BitwiseNot(dilated, &final)
	return final
}

func findLargestContourRect(binary gocv.Mat) (*RotatedRect, float64) {
	contours := gocv.FindContours(binary, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()
	
	var largestArea float64
	var largestRect *RotatedRect
	
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		area := gocv.ContourArea(contour)
		
		if area > largestArea {
			largestArea = area
			rotRect := gocv.MinAreaRect(contour)
			largestRect = &RotatedRect{
				Center: Point2f{X: float32(rotRect.Center.X), Y: float32(rotRect.Center.Y)},
				Size:   Point2f{X: float32(rotRect.Width), Y: float32(rotRect.Height)},
				Angle:  rotRect.Angle,
			}
		}
		contour.Close()
	}
	
	return largestRect, largestArea
}

func normalizeRectRotation(rawRects []*RotatedRect) []*RotatedRect {
	var rects []*RotatedRect
	for _, rect := range rawRects {
		newRect := &RotatedRect{
			Center: rect.Center,
			Size:   rect.Size,
			Angle:  rect.Angle,
		}
		
		if newRect.Angle < -45 {
			newRect.Size = Point2f{X: rect.Size.Y, Y: rect.Size.X}
			newRect.Angle = rect.Angle + 90
		}
		rects = append(rects, newRect)
	}
	return rects
}

func medianRect(rects []*RotatedRect) *RotatedRect {
	if len(rects) == 0 {
		return nil
	}
	
	normalized := normalizeRectRotation(rects)
	
	// Sort by area
	sort.Slice(normalized, func(i, j int) bool {
		areaI := float64(normalized[i].Size.X * normalized[i].Size.Y)
		areaJ := float64(normalized[j].Size.X * normalized[j].Size.Y)
		return areaI < areaJ
	})
	
	// Calculate medians
	var centerX, centerY, sizeX, sizeY, angles []float64
	for _, r := range normalized {
		centerX = append(centerX, float64(r.Center.X))
		centerY = append(centerY, float64(r.Center.Y))
		sizeX = append(sizeX, float64(r.Size.X))
		sizeY = append(sizeY, float64(r.Size.Y))
		angles = append(angles, r.Angle)
	}
	
	return &RotatedRect{
		Center: Point2f{X: float32(median(centerX)), Y: float32(median(centerY))},
		Size:   Point2f{X: float32(median(sizeX)), Y: float32(median(sizeY))},
		Angle:  median(angles),
	}
}

func correctAspectRatio(rect *RotatedRect, targetRatio, maxDifference float64) (*RotatedRect, bool) {
	size := rect.Size
	aspectRatio := math.Max(float64(size.X), float64(size.Y)) / math.Min(float64(size.X), float64(size.Y))
	aspectError := targetRatio - aspectRatio
	
	// Factor out orientation to simplify logic
	var rectWidth, rectHeight float32
	var widthIsX bool
	
	if size.X == math.Max(float64(size.X), float64(size.Y)) {
		rectWidth = size.X
		rectHeight = size.Y
		widthIsX = true
	} else {
		rectHeight = size.X
		rectWidth = size.Y
		widthIsX = false
	}
	
	// Only attempt to correct aspect ratio where the ROI is roughly right already
	if math.Abs(aspectError) > maxDifference {
		return rect, false
	}
	
	// Adjust dimensions
	if aspectRatio > targetRatio {
		if verbose {
			fmt.Fprintf(os.Stderr, "ratio too large %f\n", aspectError)
		}
		rectWidth = rectHeight * float32(targetRatio)
	} else if aspectRatio < targetRatio {
		if verbose {
			fmt.Fprintf(os.Stderr, "ratio too small %f\n", aspectError)
		}
		rectHeight = rectWidth / float32(targetRatio)
	}
	
	// Apply new width/height in the original orientation
	var newSize Point2f
	if widthIsX {
		newSize = Point2f{X: rectWidth, Y: rectHeight}
	} else {
		newSize = Point2f{X: rectHeight, Y: rectWidth}
	}
	
	newRect := &RotatedRect{
		Center: rect.Center,
		Size:   newSize,
		Angle:  rect.Angle,
	}
	
	return newRect, true
}

func calculateCropCoordinates(rect *RotatedRect, imgHeight, imgWidth int) (float64, float64, float64, float64) {
	// Get box points from rotated rectangle
	cos := math.Cos(rect.Angle * math.Pi / 180)
	sin := math.Sin(rect.Angle * math.Pi / 180)
	
	halfW := float64(rect.Size.X) / 2
	halfH := float64(rect.Size.Y) / 2
	
	cx := float64(rect.Center.X)
	cy := float64(rect.Center.Y)
	
	// Calculate the four corners of the rotated rectangle
	points := []image.Point{
		{X: int(cx + halfW*cos - halfH*sin), Y: int(cy + halfW*sin + halfH*cos)},
		{X: int(cx - halfW*cos - halfH*sin), Y: int(cy - halfW*sin + halfH*cos)},
		{X: int(cx - halfW*cos + halfH*sin), Y: int(cy - halfW*sin - halfH*cos)},
		{X: int(cx + halfW*cos + halfH*sin), Y: int(cy + halfW*sin - halfH*cos)},
	}
	
	// Find bounding box
	var left, right, top, bottom []int
	for _, point := range points {
		if float64(point.X) > cx {
			right = append(right, point.X)
		} else {
			left = append(left, point.X)
		}
		
		if float64(point.Y) > cy {
			bottom = append(bottom, point.Y)
		} else {
			top = append(top, point.Y)
		}
	}
	
	cropRight := float64(minInt(right)) / float64(imgWidth)
	cropLeft := float64(maxInt(left)) / float64(imgWidth)
	cropBottom := float64(minInt(bottom)) / float64(imgHeight)
	cropTop := float64(maxInt(top)) / float64(imgHeight)
	
	return cropLeft, cropRight, cropTop, cropBottom
}

func enforce32AspectRatio(cropLeft, cropRight, cropTop, cropBottom float64, imgWidth, imgHeight int) (float64, float64, float64, float64) {
	// Convert normalized crop bounds to pixel units
	x0 := cropLeft * float64(imgWidth)
	x1 := cropRight * float64(imgWidth)
	y0 := cropTop * float64(imgHeight)
	y1 := cropBottom * float64(imgHeight)
	
	// Current crop width/height in pixels
	w := math.Max(0.0, x1-x0)
	h := math.Max(0.0, y1-y0)
	if w <= 0.0 || h <= 0.0 {
		return cropLeft, cropRight, cropTop, cropBottom
	}
	
	r := w / h
	r32 := 3.0 / 2.0
	r23 := 2.0 / 3.0
	
	// Choose the nearest target ratio
	var target float64
	if math.Abs(r-r32) <= math.Abs(r-r23) {
		target = r32
	} else {
		target = r23
	}
	
	// Option A: keep height, reduce width to target
	wKeepH := math.Min(w, target*h)
	areaA := wKeepH * h
	
	// Option B: keep width, reduce height to target
	hKeepW := math.Min(h, w/target)
	areaB := w * hKeepW
	
	// Pick option that preserves larger area
	var decision string
	if areaA >= areaB {
		newW := wKeepH
		deltaW := w - newW
		x0 += deltaW / 2.0
		x1 -= deltaW / 2.0
		decision = "reduce-width"
	} else {
		newH := hKeepW
		deltaH := h - newH
		y0 += deltaH / 2.0
		y1 -= deltaH / 2.0
		decision = "reduce-height"
	}
	
	// Convert back to normalized [0,1]
	cropLeft = math.Max(0.0, math.Min(1.0, x0/float64(imgWidth)))
	cropRight = math.Max(0.0, math.Min(1.0, x1/float64(imgWidth)))
	cropTop = math.Max(0.0, math.Min(1.0, y0/float64(imgHeight)))
	cropBottom = math.Max(0.0, math.Min(1.0, y1/float64(imgHeight)))
	
	if verbose {
		newWPx := math.Max(0.0, (cropRight-cropLeft)*float64(imgWidth))
		newHPx := math.Max(0.0, (cropBottom-cropTop)*float64(imgHeight))
		var newR float64
		if newHPx == 0 {
			newR = math.Inf(1)
		} else {
			newR = newWPx / newHPx
		}
		fmt.Fprintf(os.Stderr, "aspect enforce (px): current= %f target= %f decision= %s new_ratio= %f\n", r, target, decision, newR)
	}
	return cropLeft, cropRight, cropTop, cropBottom
}

func shrinkCropUniform(cropLeft, cropRight, cropTop, cropBottom, percent float64) (float64, float64, float64, float64) {
	width := math.Max(0.0, cropRight-cropLeft)
	height := math.Max(0.0, cropBottom-cropTop)
	if width <= 0.0 || height <= 0.0 {
		return cropLeft, cropRight, cropTop, cropBottom
	}
	
	scale := math.Max(0.0, 1.0-percent)
	cx := (cropLeft + cropRight) / 2.0
	cy := (cropTop + cropBottom) / 2.0
	halfW := (width * scale) / 2.0
	halfH := (height * scale) / 2.0
	
	newLeft := cx - halfW
	newRight := cx + halfW
	newTop := cy - halfH
	newBottom := cy + halfH
	
	// Clamp
	newLeft = math.Max(0.0, math.Min(1.0, newLeft))
	newRight = math.Max(0.0, math.Min(1.0, newRight))
	newTop = math.Max(0.0, math.Min(1.0, newTop))
	newBottom = math.Max(0.0, math.Min(1.0, newBottom))
	
	return newLeft, newRight, newTop, newBottom
}

func drawDebugOverlays(img gocv.Mat, rawRect, insetRect, rect *RotatedRect) {
	// Draw original detected area in blue
	drawRotatedRect(img, rawRect, color.RGBA{255, 0, 0, 255}, 1)
	
	// Draw inset area in cyan
	drawRotatedRect(img, insetRect, color.RGBA{0, 255, 255, 255}, 1)
	
	// Draw adjusted aspect ratio area in green
	drawRotatedRect(img, rect, color.RGBA{0, 255, 0, 255}, 2)
	
	// Draw center point
	center := image.Point{X: int(rect.Center.X), Y: int(rect.Center.Y)}
	gocv.Circle(&img, center, 3, color.RGBA{0, 255, 0, 255}, 3)
}

func drawRotatedRect(img gocv.Mat, rect *RotatedRect, clr color.RGBA, thickness int) {
	if rect == nil {
		return
	}
	
	// Calculate the four corners of the rotated rectangle
	cos := math.Cos(rect.Angle * math.Pi / 180)
	sin := math.Sin(rect.Angle * math.Pi / 180)
	
	halfW := float64(rect.Size.X) / 2
	halfH := float64(rect.Size.Y) / 2
	
	cx := float64(rect.Center.X)
	cy := float64(rect.Center.Y)
	
	points := []image.Point{
		{X: int(cx + halfW*cos - halfH*sin), Y: int(cy + halfW*sin + halfH*cos)},
		{X: int(cx - halfW*cos - halfH*sin), Y: int(cy - halfW*sin + halfH*cos)},
		{X: int(cx - halfW*cos + halfH*sin), Y: int(cy - halfW*sin - halfH*cos)},
		{X: int(cx + halfW*cos + halfH*sin), Y: int(cy + halfW*sin - halfH*cos)},
	}
	
	// Draw lines between consecutive points
	for i := 0; i < len(points); i++ {
		start := points[i]
		end := points[(i+1)%len(points)]
		gocv.Line(&img, start, end, clr, thickness)
	}
}

func writeCropData(filename string, data []float64) {
	file, err := os.Create(filename)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Failed to create %s: %v\n", filename, err)
		}
		return
	}
	defer file.Close()
	
	for _, value := range data {
		fmt.Fprintf(file, "%f\r\n", value)
	}
}

// Utility functions

func median(values []float64) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func maxInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	var imageFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		path := filepath.Join(dir, entry.Name())
		if isImageFile(path) {
			imageFiles = append(imageFiles, path)
		}
	}
	
	return imageFiles, nil
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || 
		   ext == ".tif" || ext == ".tiff" || ext == ".bmp" || ext == ".webp"
}