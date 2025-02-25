package imagereader

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"github.com/chai2010/webp"
	"github.com/gen2brain/avif"
	"golang.org/x/image/tiff"
)

func Decode(reader io.Reader) (image.Image, string, error) {
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
