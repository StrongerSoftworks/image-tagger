package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/StrongerSoftworks/image-tagger/imagetiler"
)

type OllavaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Stream bool     `json:"stream"`
	Images []string `json:"images"`
}

type OllavaResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	DoneReason         string    `json:"done_reason"`
	TotalDuration      int64     `json:"total_duration"`
	LoadDuration       int       `json:"load_duration"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	PromptEvalDuration int       `json:"prompt_eval_duration"`
	EvalCount          int       `json:"eval_count"`
	EvalDuration       int64     `json:"eval_duration"`
}

type ImageData struct {
	File string   `json:"file"`
	Alt  string   `json:"alt"`
	Tags []string `json:"tags"`
}

const defaultOllamaURL = "http://localhost:11434/api/generate"
const fileRoot = "images"
const model = "llava:13b"

func main() {
	// Command line arguments
	var imageListFilePath, outputPath string
	var cropWidth, cropHeight int
	var saveCropped bool
	var isLocalImageSource bool

	flag.StringVar(&imageListFilePath, "path", "", "Path to the image file or web URL")
	flag.StringVar(&outputPath, "out", "out", "Path to save the tiled images")
	flag.IntVar(&cropWidth, "width", 672, "Crop width (default: 672)")
	flag.IntVar(&cropHeight, "height", 672, "Crop height (default: 672)")
	flag.BoolVar(&saveCropped, "save", false, "Save cropped images (default: false). For debugging purposes. Images that are saved are not automatically deletd by image-tagger.")
	flag.BoolVar(&isLocalImageSource, "local", false, "Specify if the source is a local (default: true)")
	flag.Parse()

	if imageListFilePath == "" {
		fmt.Println("Error: Image path or URL must be provided.")
		return
	}

	imageData := []ImageData{}
	start := time.Now()

	// read tags
	tags := readTags("tags.txt")
	prompt := fmt.Sprintf("List every vehicle part you see in this image of a vehicle as a comma separated list. Only include parts from this list: %s. If the part is not certain to be in the image then to not list that part. Only list part that are easily discernable in the image and it is certain that the part is in the image.", tags)

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

		base64Images := imagetiler.MakeImageTiles(imagetiler.ImageTileOptions{
			IsLocalImage: isLocalImageSource,
			SaveCropped:  saveCropped,
			ImagePath:    imagePath,
			OutputDir:    outputPath,
			CropWidth:    cropWidth,
			CropHeight:   cropHeight})

		imageTags := tagsFromCroppedImages(base64Images)

		imageData = append(imageData, ImageData{File: imagePath, Alt: "", Tags: imageTags})
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	err = writeImageData(imageData)
	if err != nil {
		log.Fatalf("Did not complete writing data, exiting with error: %v", err)
	}
	err = writeAllTags(imageData)
	if err != nil {
		log.Fatalf("Did not complete writing tags, exiting with error: %v", err)
	}

	fmt.Printf("Completed in %v", time.Since(start))
}

func writeImageData(imageData []ImageData) error {
	file, err := os.Create(fmt.Sprintf("%s/image_tags.json", fileRoot))
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return err
	}
	defer file.Close()

	// Encode the slice to JSON and write it to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Optional: Pretty-print the JSON with indentation
	if err := encoder.Encode(imageData); err != nil {
		fmt.Printf("Error encoding to JSON: %v\n", err)
		return err
	}

	fmt.Println("Data successfully written to image_tags.json")
	return nil
}

func writeAllTags(imageData []ImageData) error {
	// Collect all tags into a single string array
	tagSet := make(map[string]struct{})
	for _, data := range imageData {
		for _, tag := range data.Tags {
			tagSet[tag] = struct{}{} // Use a map to avoid duplicates
		}
	}

	// Convert the map keys to a slice
	allTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}

	// Create the output file
	file, err := os.Create(fmt.Sprintf("%s/all_tags.json", fileRoot))
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return err
	}
	defer file.Close()

	// Encode the tags slice to JSON and write it to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Optional: Pretty-print the JSON with indentation
	if err := encoder.Encode(allTags); err != nil {
		fmt.Printf("Error encoding to JSON: %v\n", err)
		return err
	}

	fmt.Println("Tags successfully written to all_tags.json")
	return nil
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

func tagsFromCroppedImages(prompt string, base64Images []string) []string {
	tags := []string{}
	for _, base64Image := range base64Images {
		req := OllavaRequest{
			Model:  model,
			Stream: false,
			Prompt: prompt,
			Images: []string{base64Image},
		}
		resp, err := talkToOllama(defaultOllamaURL, req)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(resp.Response)
		tags = append(tags, strings.Split(resp.Response, ", ")...)
	}
	return tags
}

func talkToOllama(url string, ollamaReq OllavaRequest) (*OllavaResponse, error) {
	js, err := json.Marshal(&ollamaReq)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	ollamaResp := OllavaResponse{}
	err = json.NewDecoder(httpResp.Body).Decode(&ollamaResp)
	return &ollamaResp, err
}
