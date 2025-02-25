package imageloader

import (
	"fmt"
	"image"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/StrongerSoftworks/image-tagger/internal/imagereader"
)

func LoadImage(imagePath string) image.Image {
	// Load the image
	var img image.Image
	var err error
	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		img, _, err = loadImageFromURL(imagePath)
	} else if matched, _ := regexp.MatchString(`^\w+://`, imagePath); matched {
		slog.Error("Unknown protocol", "protocol", imagePath)
		return nil
	} else {
		img, _, err = loadImageFromFile(imagePath)
	}

	if err != nil {
		slog.Error("Error loading image", "error", err)
		return nil
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

	return imagereader.Decode(file)
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

	return imagereader.Decode(resp.Body)
}
