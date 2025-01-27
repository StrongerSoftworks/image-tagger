package imagetiler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/gen2brain/avif"
	"golang.org/x/image/tiff"
)

type ImageTileOptions struct {
	IsLocalImage bool
	SaveCropped  bool
	ImagePath    string
	OutputDir    string
	CropWidth    int
	CropHeight   int
}

func MakeImageTiles(options ImageTileOptions) []string {
	// Load the image
	var img image.Image
	var err error
	if options.IsLocalImage {
		img, _, err = loadImageFromFile(options.ImagePath)
	} else {
		img, _, err = loadImageFromURL(options.ImagePath)
	}
	if err != nil {
		log.Fatalf("Error loading image: %v\n", err)
		return nil
	}

	// Crop the image
	croppedImages := cropImage(img, options.CropWidth, options.CropHeight)

	var base64images []string = make([]string, len(croppedImages))
	// Save or process the cropped images
	for i, cropped := range croppedImages {
		if options.SaveCropped {
			saveCroppedImage(cropped, options.ImagePath, options.OutputDir, i)
		}

		// Base64 encode the cropped image
		base64images[i] = encodeImageToBase64(cropped)
	}

	return base64images
}

func loadImageFromFile(filePath string) (image.Image, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	return image.Decode(file)
	// return decodeImage(bufio.NewReader(file))
}

func loadImageFromURL(url string) (image.Image, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Error fetching image: HTTP %d\n", resp.StatusCode)
	}

	return image.Decode(resp.Body)
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

	heightStep := int(float64(cropHeight) * 0.667)
	widthStep := int(float64(cropWidth) * 0.667)
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
		fmt.Printf("Error creating output dir: %v\n", err)
		return
	}

	newFileName := fmt.Sprintf("%s/%s-%d.png", outputDir, base, index)
	file, err := os.Create(newFileName)
	if err != nil {
		fmt.Printf("Error saving cropped image: %v\n", err)
		return
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		fmt.Printf("Error encoding cropped image: %v\n", err)
	}
}

func encodeImageToBase64(img image.Image) string {
	buffer := new(bytes.Buffer)
	err := png.Encode(buffer, img)
	if err != nil {
		fmt.Printf("Error encoding image to Base64: %v\n", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
