package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/StrongerSoftworks/image-tagger/internal/imageloader"
	"github.com/StrongerSoftworks/image-tagger/internal/imagetiler"
	"github.com/ollama/ollama/api"
)

type ImageData struct {
	File        string           `json:"file"`
	Processed   time.Time        `json:"processed"`
	Subject     string           `json:"subject"`
	Description string           `json:"description"`
	Tags        []VisionModelTag `json:"tags"`
}

type VisionModelTag struct {
	Object     string `json:"object"`
	Confidence int    `json:"confidence"`
}

type VisionModelTags struct {
	Subject string           `json:"subject"`
	Tags    []VisionModelTag `json:"tags"`
}

type VisionModelSummary struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

var visionModel string

const confidenceThreshold = 50

func main() {
	// Command line arguments
	var imageFilePath, tagsFilePath, outputPath string
	var cropSize, cropWidth, cropHeight int
	var mode string
	var saveCropped bool
	var debugMode bool
	var help bool

	flag.StringVar(&imageFilePath, "image", "", "Path to the image to process")
	flag.StringVar(&tagsFilePath, "tags_path", "", "Path to the tags file")
	flag.StringVar(&outputPath, "out", "out", "Path to save the tiled images")
	flag.StringVar(&visionModel, "vision_model", "llava:13b", "Model to use for vision (default: llava:13b)")
	flag.IntVar(&cropWidth, "width", 672, "Resize width (default: 672)")
	flag.IntVar(&cropHeight, "height", 672, "Resize height (default: 672)")
	flag.IntVar(&cropSize, "crop", 672, "Used with mode=tile. Crop width and height. Uses max_crops to create smaller images from the image and sending each image to the vision model (default: 512)")
	flag.StringVar(&mode, "mode", "tile", "'fit' or 'tile'. 'fit' will resize the image to fit the given width and height. 'tile' will resize the image to fit \"crop\" x \"crop\" then process the image in 4 tiles with max width and height of \"crop\".")
	flag.BoolVar(&saveCropped, "save", false, "Save cropped images. For debugging purposes. Images that are saved are not automatically deleted by image-tagger.")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug mode")
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

	if imageFilePath == "" {
		fmt.Println("Image file path or URL must be provided.")
		flag.PrintDefaults()
		return
	}

	start := time.Now()

	// read tags
	desiredTags := readTagsFilter(tagsFilePath)

	slog.Info("Processing image", "imagePath", imageFilePath)

	options := imagetiler.Options{
		SaveCropped: saveCropped,
		ImagePath:   imageFilePath,
		OutputDir:   outputPath,
		Width:       cropWidth,
		Height:      cropHeight,
		CropSize:    cropSize,
		Mode:        imagetiler.Mode(mode),
	}

	img := imageloader.LoadImage(options.ImagePath)
	images := imagetiler.MakeImageTiles(options, img)

	ollamaClient, err := api.ClientFromEnvironment()
	if err != nil {
		slog.Error("Error creating ollama client", "error", err)
		return
	}

	var imagesData []api.ImageData = make([]api.ImageData, len(images))
	for idx, image := range images {
		imagesData[idx] = image
	}

	summary := generateImageSummary(ollamaClient, imagesData)
	summaryTags := generateImageTags(ollamaClient, imagesData, summary.Subject, desiredTags)
	imageDataWithTags := ImageData{
		File:        filepath.Base(imageFilePath),
		Processed:   time.Now(),
		Subject:     summary.Subject,
		Description: summary.Description,
		Tags:        summaryTags,
	}

	jsonData, err := json.Marshal(imageDataWithTags)
	if err != nil {
		slog.Error("Error marshaling tags to JSON", "error", err)
		return
	}

	// Write JSON to file with image name as prefix
	jsonFileName := fmt.Sprintf("%s_tags.json", filepath.Base(imageFilePath))
	err = os.WriteFile(path.Join(outputPath, jsonFileName), jsonData, 0644)
	if err != nil {
		slog.Error("Error writing tags to file", "error", err)
		return
	}

	slog.Info("Completed", "time", time.Since(start))
}

// readTagsFilter reads the tags file and returns a string of tags
func readTagsFilter(filePath string) []string {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Error opening tags file", "error", err)
		return []string{}
	}
	defer file.Close()

	// Create a Scanner to read the file
	var tags []string
	fileContents, err := io.ReadAll(file)
	if err != nil {
		slog.Error("Error reading tags file", "error", err)
		return []string{}
	}
	err = json.Unmarshal(fileContents, &tags)
	if err != nil {
		slog.Error("Error unmarshalling tags", "error", err)
		return []string{}
	}

	return tags
}

func generateImageSummary(ollamaClient *api.Client, imagesData []api.ImageData) VisionModelSummary {
	var wg sync.WaitGroup
	results := make(chan VisionModelSummary, 1)

	wg.Add(1)
	go sendVisionSummaryRequest(ollamaClient, imagesData, &wg, results)

	wg.Wait()
	close(results)

	return <-results
}

// generateImageTags sends a generate request to the vision model running on the ollama client
func generateImageTags(ollamaClient *api.Client, imagesData []api.ImageData, subject string, desiredTags []string) []VisionModelTag {
	var wg sync.WaitGroup
	results := make(chan VisionModelTags, 1)

	wg.Add(1)
	go sendVisionTagsRequest(ollamaClient, imagesData, subject, desiredTags, &wg, results)

	wg.Wait()
	close(results)

	return collectUniqueTags(results)
}

func sendVisionSummaryRequest(ollamaClient *api.Client, imagesData []api.ImageData, wg *sync.WaitGroup, summaries chan<- VisionModelSummary) {
	prompt := "You are a professional SEO specialist. Analyze the provided image and provide:" +
		"    subject: The main subject of the image as a single word. The subject can be an object or improper noun." +
		"    description: A short description of the image no longer than 20 words." +
		" No introductions, explanations, or extra text." +
		" Respond using JSON."

	request := &api.GenerateRequest{
		Model:  visionModel,
		Prompt: prompt,
		Stream: new(bool),
		Images: imagesData,
		Format: []byte(`{
			"type": "object",
			"properties": {
				"subject": { "type": "string" },
				"description": { "type": "string" }
			},
			"required": [
				"subject", "description"
			]
		}`),
	}

	responseHandler := func(response api.GenerateResponse) error {
		slog.Debug("Summary response", "response", response.Response)
		defer wg.Done()

		var imageSummary VisionModelSummary
		err := json.Unmarshal([]byte(response.Response), &imageSummary)
		if err != nil {
			slog.Error("Error unmarshalling summary", "error", err)
			return err
		}
		summaries <- imageSummary

		return nil
	}

	slog.Debug("Sending summary request", "request", request.Prompt)
	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		slog.Error("Error sending generate request to ollama", "error", err)
		wg.Done()
	}
}

// sendVisionRequest sends a generate request to the vision model running on the ollama client
func sendVisionTagsRequest(ollamaClient *api.Client, imageData []api.ImageData, subject string, desiredTags []string, wg *sync.WaitGroup, summaries chan<- VisionModelTags) {
	objectInstruction := "that are visible in the image"
	if len(desiredTags) > 0 {
		objectInstruction = fmt.Sprintf("from the following list: [%s]", strings.Join(desiredTags, ", "))
	}
	prompt := fmt.Sprintf("You are assembling a list of tags for a web application that will be used for browsing images and filtering images by tags."+
		" Analyze the provided image of a %s and identify the objects %s."+
		" If an object is found, provide: "+
		"    object: An object from the list of objects."+
		"    confidence: A confidence level number between 0 and 100 based on clarity, visibility, and similarity to known references."+
		" The object should clearly be visible in the image and you must me confident that the object is correctly identified."+
		" No introductions, explanations, or extra text."+
		" Respond using JSON.", subject, objectInstruction)

	request := &api.GenerateRequest{
		Model:  visionModel,
		Prompt: prompt,
		Stream: new(bool),
		Images: imageData,
		Format: []byte(`{
			"type": "object",
			"properties": {
				"tags": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"object": {
								"type": "string"
							},
							"confidence": {
								"type": "number"
							}
						},
						"required": ["object", "confidence"]
					}
				}
			},
			"required": [
				"tags"
			]
		}`),
	}

	responseHandler := func(response api.GenerateResponse) error {
		slog.Debug("Tag response", "response", response.Response)
		defer wg.Done()

		var imageSummary VisionModelTags
		err := json.Unmarshal([]byte(response.Response), &imageSummary)
		if err != nil {
			slog.Error("Error unmarshalling tags", "error", err)
			return err
		}
		summaries <- imageSummary

		return nil
	}

	slog.Debug("Sending tag request", "request", request.Prompt)
	err := ollamaClient.Generate(context.Background(), request, responseHandler)
	if err != nil {
		slog.Error("Error sending generate request to ollama", "error", err)
		wg.Done()
	}
}

// collectUniqueTags filters tags with confidence greater than the threshold and ensures uniqueness.
func collectUniqueTags(summaryChan <-chan VisionModelTags) []VisionModelTag {
	tagMap := make(map[string]VisionModelTag)

	for summary := range summaryChan { // Read from the channel until it's closed
		for _, tag := range summary.Tags {
			if tag.Confidence >= confidenceThreshold {
				// Store the tag in the map, keeping the highest confidence value
				if existingTag, exists := tagMap[tag.Object]; !exists || tag.Confidence > existingTag.Confidence {
					tagMap[tag.Object] = tag
				}
			}
		}
	}

	// Convert map values to a slice
	uniqueTags := make([]VisionModelTag, 0, len(tagMap))
	for _, tag := range tagMap {
		uniqueTags = append(uniqueTags, tag)
	}

	return uniqueTags
}
