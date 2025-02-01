package imagetiler

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"log/slog"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	"golang.org/x/image/tiff"
)

type Mode string

const (
	ModeFit  Mode = "fit"
	ModeTile Mode = "tile"
)

type Options struct {
	MaxImagePixels int
	SaveCropped    bool
	ImagePath      string
	OutputDir      string
	Width          int
	Height         int
	Mode           Mode
}

func MakeImageTiles(options Options) [][]byte {
	// Load the image
	var img image.Image
	var err error
	if strings.HasPrefix(options.ImagePath, "http://") || strings.HasPrefix(options.ImagePath, "https://") {
		img, _, err = loadImageFromURL(options.ImagePath)
	} else if matched, _ := regexp.MatchString(`^\w+://`, options.ImagePath); matched {
		slog.Error("Unknown protocol", "protocol", options.ImagePath)
		return nil
	} else {
		img, _, err = loadImageFromFile(options.ImagePath)
	}

	if err != nil {
		slog.Error("Error loading image", "error", err)
		return nil
	}

	// Resize image if it goes beyond the configured max size
	img = resizeImage(img, options)

	// Crop the image
	croppedImages := cropImage(img, options.Width, options.Height)

	var imageData [][]byte = make([][]byte, len(croppedImages))
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

func resizeImage(img image.Image, options Options) image.Image {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	if options.Mode == ModeFit {
		slog.Debug("Resizing image", "from", fmt.Sprintf("%d x %d", imgWidth, imgHeight), "to", fmt.Sprintf("%d x %d", options.Width, options.Height))
		return imaging.Fit(img, options.Width, options.Height, imaging.Lanczos)
	}

	imgPixels := imgWidth * imgHeight
	if imgPixels > options.MaxImagePixels {
		ratio := float64(imgWidth) / float64(imgHeight)
		scale := math.Sqrt(float64(imgPixels) / float64(options.MaxImagePixels))
		newHeight := int(math.Floor(float64(imgHeight) / scale))
		newWidth := int(math.Floor(ratio * float64(imgHeight) / scale))
		slog.Debug("Resizing image", "from", fmt.Sprintf("%d x %d", imgWidth, imgHeight), "to", fmt.Sprintf("%d x %d", newWidth, newHeight))
		img = imaging.Fit(img, newWidth, newHeight, imaging.Lanczos)

	}
	return img
}

func loadImageFromFile(filePath string) (image.Image, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Error loading image from file", "error", err)
		return nil, "", err
	}
	defer file.Close()

	return decodeImage(file)
}

func loadImageFromURL(url string) (image.Image, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("Error fetching image from URL", "error", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Error fetching image from URL", "error", fmt.Errorf("error fetching image: HTTP %d", resp.StatusCode))
		return nil, "", fmt.Errorf("error fetching image: HTTP %d", resp.StatusCode)
	}

	return decodeImage(resp.Body)
}

func decodeImage(reader io.Reader) (image.Image, string, error) {
	// Detect format using the standard image package
	img, format, err := image.Decode(reader)
	if err == nil {
		return img, format, nil
	}

	img, err = jpeg.Decode(reader)
	if err == nil {
		return img, "jpeg", nil
	}

	img, err = webp.Decode(reader)
	if err == nil {
		return img, "webp", nil
	}

	img, err = avif.Decode(reader)
	if err == nil {
		return img, "avif", nil
	}

	img, err = tiff.Decode(reader)
	if err == nil {
		return img, "tiff", nil
	}

	// If no decoder could handle the data, return an error
	return nil, "", fmt.Errorf("unsupported image format or corrupted image")
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
