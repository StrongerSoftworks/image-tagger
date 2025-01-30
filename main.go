package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StrongerSoftworks/image-tagger/imagetiler"
	"github.com/ollama/ollama/api"
)

type ImageData struct {
	File string   `json:"file"`
	Alt  string   `json:"alt"`
	Tags []string `json:"tags"`
}

const visionModel string = "llava:13b"      //"llava-llama3"
const reasoningModel string = "llama3.2:3b" // TODO try mistral

func main() {
	// Command line arguments
	var imageListFilePath, tagsFilePath, outputPath string
	var cropWidth, cropHeight, maxPixels int
	var mode string
	var saveCropped bool
	var isLocalImageSource bool

	flag.StringVar(&imageListFilePath, "images_path", "", "Path to the file that contains a list of image file paths")
	flag.StringVar(&tagsFilePath, "tags_path", "", "Path to the tags file")
	flag.StringVar(&outputPath, "out", "out", "Path to save the tiled images")
	flag.IntVar(&cropWidth, "width", 672, "Crop width (default: 672)")
	flag.IntVar(&cropHeight, "height", 672, "Crop height (default: 672)")
	flag.IntVar(&maxPixels, "max_pixels", 2000000, "Max pixels for source image. The source image will be resized if it is larger than configured max pixels (default: 2000000)")
	flag.StringVar(&mode, "mode", "fit", "'fit' or 'tile'. 'fit' will resize the image to fit the given width and height. 'tile' will resize the image to fit the given max pixels then process the image in tiles defined by width and height. (default: fit)")
	flag.BoolVar(&saveCropped, "save", false, "Save cropped images (default: false). For debugging purposes. Images that are saved are not automatically deleted by image-tagger.")
	flag.BoolVar(&isLocalImageSource, "local", false, "Specify if the source is a local (default: true)")
	flag.Parse()

	if imageListFilePath == "" {
		fmt.Println("Error: Image path or URL must be provided.")
		return
	}

	start := time.Now()

	// read tags
	tags := readTags(tagsFilePath)
	prompt := "Describe every part, component, control, feature or item in this photo. Only include items that are present and visible in the image. Ignore items that are not present or not visible in the image and do not include them in the description."

	// read file list
	file, err := os.Open(imageListFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		imagePath := scanner.Text()
		fmt.Println(imagePath)

		images := imagetiler.MakeImageTiles(imagetiler.ImageTileOptions{
			IsLocalImage:   isLocalImageSource,
			SaveCropped:    saveCropped,
			ImagePath:      imagePath,
			OutputDir:      outputPath,
			Width:          cropWidth,
			Height:         cropHeight,
			MaxImagePixels: maxPixels,
			Mode:           mode,
		})

		getImageTags(prompt, tags, images, imagePath)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	fmt.Printf("Completed in %v", time.Since(start))
}

func readTags(filePath string) string {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening tags file: %v\n", err)
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
		fmt.Printf("Error reading tags file: %v\n", err)
	}

	return tags
}

func getImageTags(prompt string, desiredTags string, images [][]byte, imageFile string) {
	ollamaClient, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	for i, imageData := range images {
		sendVisionRequest(i, prompt, imageData, desiredTags, imageFile, ollamaClient)
	}
}

func sendVisionRequest(index int, prompt string, imageData []byte, desiredTags string, imageFile string, ollamaClient *api.Client) {
	request := &api.GenerateRequest{
		Model:  visionModel,
		Prompt: prompt,
		Stream: new(bool),
		Images: []api.ImageData{imageData},
	}
	responseHandler := func(response api.GenerateResponse) error {
		fmt.Println(response.Response)
		// send a chat request to get a list of tags from the description
		return sendSummaryRequest(index, desiredTags, response.Response, imageFile, ollamaClient)
	}
	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		log.Fatalf("Error sending generate request to ollama: %s", err)
	}
}

func sendSummaryRequest(index int, desiredTags string, imageDescription string, imageFile string, ollamaClient *api.Client) error {
	request := &api.GenerateRequest{
		Model:  reasoningModel,
		Prompt: fmt.Sprintf("Using this list: `%s`, extract and list only the items from the provided list that also are mentioned or described in the following content. Respond with a single comma-separated list of one or two-word phrases exactly as they appear in the provided list. No introductions, explanations, or extra text. Content: %s", desiredTags, imageDescription),
		Stream: new(bool),
	}
	responseHandler := func(response api.GenerateResponse) error {
		fmt.Println(response.Response)
		tags := strings.Split(strings.ToLower(response.Response), ", ")

		jsonData, err := json.Marshal(tags)
		if err != nil {
			log.Printf("Error marshaling tags to JSON: %v", err)
			return err
		}

		// Write JSON to file with image name as prefix
		jsonFileName := fmt.Sprintf("%s_%d_tags.json", imageFile[:len(imageFile)-len(filepath.Ext(imageFile))], index)
		err = os.WriteFile(jsonFileName, jsonData, 0644)
		if err != nil {
			log.Printf("Error writing tags to file: %v", err)
			return err
		}
		return nil
	}
	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		log.Fatalf("Error sending generate request to ollama: %s", err)
		return err
	}
	return nil
}
