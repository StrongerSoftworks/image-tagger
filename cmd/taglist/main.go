package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StrongerSoftworks/image-tagger/internal/imagetiler"
	"github.com/ollama/ollama/api"
)

type ImageData struct {
	File string   `json:"file"`
	Alt  string   `json:"alt"`
	Tags []string `json:"tags"`
}

var visionModel string
var summaryModel string

// TODO try a merge model such as
// https://ollama.com/library/bakllava (Mistral + LlaVa)
// https://ollama.com/library/llava-llama3 (Llama3 + LlaVa)

func main() {
	// Command line arguments
	var imageListFilePath, tagsFilePath, outputPath string
	var cropWidth, cropHeight, maxPixels int
	var mode string
	var saveCropped bool
	var debugMode bool
	var help bool

	flag.StringVar(&imageListFilePath, "images_path", "", "Path to the file that contains a list of image file paths")
	flag.StringVar(&tagsFilePath, "tags_path", "", "Path to the tags file")
	flag.StringVar(&outputPath, "out", "out", "Path to save the tiled images")
	flag.StringVar(&visionModel, "vision_model", "llava:13b", "Model to use for vision (default: llava:13b)")
	flag.StringVar(&summaryModel, "summary_model", "mistral:7b", "Model to use for summary (default: mistral:7b)")
	flag.IntVar(&cropWidth, "width", 672, "Crop or resize width (default: 672)")
	flag.IntVar(&cropHeight, "height", 672, "Crop or resize height (default: 672)")
	flag.IntVar(&maxPixels, "max_pixels", 2000000, "Max pixels for source image. The source image will be resized if it is larger than configured max pixels (default: 2000000)")
	flag.StringVar(&mode, "mode", "fit", "'fit' or 'tile'. 'fit' will resize the image to fit the given width and height. 'tile' will resize the image to fit the given max pixels then process the image in tiles defined by width and height. (default: fit)")
	flag.BoolVar(&saveCropped, "save", false, "Save cropped images (default: false). For debugging purposes. Images that are saved are not automatically deleted by image-tagger.")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug mode (default: false)")
	flag.BoolVar(&help, "help", false, "Show help")
	flag.Parse()

	if help {
		fmt.Println("Options:")
		flag.PrintDefaults()
		return
	}

	if debugMode {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	if imageListFilePath == "" {
		fmt.Println("Image list file path or URL must be provided.")
		flag.PrintDefaults()
		return
	}
	if tagsFilePath == "" {
		fmt.Println("Tags file path must be provided.")
		flag.PrintDefaults()
		return
	}

	start := time.Now()

	// read tags
	tags := readTagsFilter(tagsFilePath)
	prompt := "Describe every part, component, control, feature or item in this photo. Only include items that are present and visible in the image. Ignore items that are not present or not visible in the image and do not include them in the description."

	// read file list
	file, err := os.Open(imageListFilePath)
	if err != nil {
		slog.Error("Error opening file", "error", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		imagePath := scanner.Text()
		slog.Info("Processing image", "imagePath", imagePath)

		images := imagetiler.MakeImageTiles(imagetiler.Options{
			SaveCropped:    saveCropped,
			ImagePath:      imagePath,
			OutputDir:      outputPath,
			Width:          cropWidth,
			Height:         cropHeight,
			MaxImagePixels: maxPixels,
			Mode:           imagetiler.Mode(mode),
		})

		generateImageTags(prompt, tags, images, imagePath)
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Error reading file", "error", err)
	}

	slog.Info("Completed", "time", time.Since(start))
}

// readTagsFilter reads the tags file and returns a string of tags
func readTagsFilter(filePath string) string {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Error opening tags file", "error", err)
		return ""
	}
	defer file.Close()

	// Create a Scanner to read the file
	scanner := bufio.NewScanner(file)

	var tags string
	// Read the first line from the file
	if scanner.Scan() {
		tags = scanner.Text()
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		slog.Error("Error reading tags file", "error", err)
	}

	return tags
}

// generateImageTags sends a generate request to the vision model running on the ollama client
func generateImageTags(prompt string, desiredTags string, images [][]byte, imageFile string) {
	ollamaClient, err := api.ClientFromEnvironment()
	if err != nil {
		slog.Error("Error creating ollama client", "error", err)
		return
	}

	for i, imageData := range images {
		sendVisionRequest(i, prompt, imageData, desiredTags, imageFile, ollamaClient)
	}
}

// sendVisionRequest sends a generate request to the vision model running on the ollama client
func sendVisionRequest(index int, prompt string, imageData []byte, desiredTags string, imageFile string, ollamaClient *api.Client) {
	request := &api.GenerateRequest{
		Model:  visionModel,
		Prompt: prompt,
		Stream: new(bool),
		Images: []api.ImageData{imageData},
	}
	responseHandler := func(response api.GenerateResponse) error {
		slog.Debug("Vision response", "response", response.Response)
		// send a chat request to get a list of tags from the description
		return sendSummaryRequest(index, desiredTags, response.Response, imageFile, ollamaClient)
	}

	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		slog.Error("Error sending generate request to ollama", "error", err)
	}

}

// sendSummaryRequest sends a generate request to the summary model running on the ollama client
func sendSummaryRequest(index int, desiredTags string, imageDescription string, imageFilePath string, ollamaClient *api.Client) error {
	request := &api.GenerateRequest{
		Model: summaryModel,
		Prompt: fmt.Sprintf(
			"Using this list: `%s`, extract and list only the items from the provided list "+
				"that also are mentioned or described in the following content. Respond with a "+
				"single comma-separated list of one or two-word phrases exactly as they appear "+
				"in the provided list. On a second line include a summary of the content which is "+
				"the description of an image and keep the summary to less than 18 words and summarize "+
				"as if describing the subject of the image and use best practices for and img alt tag. "+
				"No introductions, explanations, or extra text. "+
				"Content: %s",
			desiredTags, imageDescription),
		Stream: new(bool),
	}
	responseHandler := func(response api.GenerateResponse) error {
		slog.Debug("Summary response", "response", response.Response)
		lines := strings.Split(response.Response, "\n")

		// verify multi line response
		if len(lines) < 2 {
			slog.Error("Summary response is not multi line", "response", response.Response)
			return fmt.Errorf("summary response is not multi line")
		}

		tags := strings.Split(strings.ToLower(lines[0]), ", ")

		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}

		imageDataWithTags := ImageData{
			File: filepath.Base(imageFilePath),
			Alt:  strings.TrimSpace(lines[len(lines)-1]), // generate adds a blank line between the tags and the summary so take the last line
			Tags: tags,
		}

		jsonData, err := json.Marshal(imageDataWithTags)
		if err != nil {
			slog.Error("Error marshaling tags to JSON", "error", err)
			return err
		}

		// Write JSON to file with image name as prefix
		jsonFileName := fmt.Sprintf("%s_%d_tags.json", imageFilePath[:len(imageFilePath)-len(filepath.Ext(imageFilePath))], index)
		err = os.WriteFile(jsonFileName, jsonData, 0644)
		if err != nil {
			slog.Error("Error writing tags to file", "error", err)
			return err
		}

		return nil
	}
	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		slog.Error("Error sending generate request to ollama", "error", err)
		return err
	}

	return nil
}
