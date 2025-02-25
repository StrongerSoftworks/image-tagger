package imagetiler

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/disintegration/imaging"
)

type Mode string

const (
	ModeFit  Mode = "fit"
	ModeTile Mode = "tile"
)

type Options struct {
	MaxCrops    int
	CropSize    int
	SaveCropped bool
	ImagePath   string
	OutputDir   string
	Width       int
	Height      int
	Mode        Mode
}

func MakeImageTiles(options Options, img image.Image) [][]byte {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	slog.Debug("Resizing image", "from", fmt.Sprintf("%d x %d", imgWidth, imgHeight), "to", fmt.Sprintf("%d x %d", options.Width, options.Height))
	resizedImage := imaging.Fit(img, options.Width, options.Height, imaging.Lanczos)

	// crop mode
	if options.Mode == ModeFit {
		buf := new(bytes.Buffer)
		err := png.Encode(buf, resizedImage)
		if err != nil {
			slog.Error("Error encoding image", "error", err)
			return nil
		}
		return [][]byte{buf.Bytes()}
	}

	// tile mode
	maxSize := int(float64(options.CropSize) * math.Floor(math.Sqrt(float64(options.MaxCrops))) * 1.5)
	slog.Debug("Resizing image", "from", fmt.Sprintf("%d x %d", imgWidth, imgHeight), "to", fmt.Sprintf("%d x %d", maxSize, maxSize))
	img = imaging.Fit(img, maxSize, maxSize, imaging.Lanczos)
	croppedImages := cropImage(img, options.Width, options.Height)

	// always include the resized full image as the first image
	croppedImages = append([]image.Image{resizedImage}, croppedImages...)

	var imageData [][]byte = make([][]byte, len(croppedImages)+1)
	// Save or process the cropped images
	for i, cropped := range croppedImages {
		if options.SaveCropped {
			saveCroppedImage(cropped, options.ImagePath, options.OutputDir, i)
		}

		buf := new(bytes.Buffer)
		err := png.Encode(buf, cropped)
		if err != nil {
			slog.Error("Error encoding image", "error", err)
			return nil
		}
		imageData[i] = buf.Bytes()

	}
	return imageData

}

func cropImage(img image.Image, cropWidth, cropHeight int) []image.Image {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	// If the image is smaller than the crop dimensions, return the original image
	if imgWidth <= cropWidth && imgHeight <= cropHeight {
		return []image.Image{img}
	}

	heightStep := int(float64(cropHeight) * 0.5)
	widthStep := int(float64(cropWidth) * 0.5)
	var croppedImages []image.Image
	for y := 0; y < imgHeight-heightStep; y += heightStep {
		for x := 0; x < imgWidth-widthStep; x += widthStep {
			// Ensure the cropping does not go out of bounds
			cropRect := image.Rect(x, y, min(x+cropWidth, imgWidth), min(y+cropHeight, imgHeight))
			cropped := img.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(cropRect)
			croppedImages = append(croppedImages, cropped)
		}
	}
	return croppedImages
}

func saveCroppedImage(img image.Image, originalPath string, outputDir string, index int) {
	ext := filepath.Ext(originalPath)
	base := strings.TrimSuffix(filepath.Base(originalPath), ext)

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		slog.Error("Error creating output dir", "error", err)
		return
	}

	newFileName := fmt.Sprintf("%s/%s-%d.png", outputDir, base, index)
	file, err := os.Create(newFileName)
	if err != nil {
		slog.Error("Error saving cropped image", "error", err)
		return
	}

	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		slog.Error("Error encoding cropped image", "error", err)
	}

}
