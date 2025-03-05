# Image Tagger

Image Tagger is a tool for tagging and getting summaries of the contents of images. It uses a vision multi model running on Ollama to generate the tags and summaries. The model it uses can be configured by command options.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Building the Project](#building-the-project)
- [Running the Project](#running-the-project)
- [Debugging the Project](#debugging-the-project)
- [Helpful Commands](#helpful-commands)

## Installation

To get started with Image Tagger, clone the repository to your local machine:

```bash
git clone https://github.com/StrongerSoftworks/image-tagger.git
cd image-tagger
```

## Prerequisites

Download and install Ollama:

- https://ollama.com/download
- https://formulae.brew.sh/formula/ollama

Get the model:

```
ollama run llava:13b
```

## Usage

```
go run cmd/tag/main.go -help
```

```
OLLAMA_HOST="http://localhost:11434"
go run cmd/tag/main.go -image image.png -tags_path tags.json -out out -mode fit -debug -save
```

## Options:

    -confidence int
        Threshold for tag confidence. Any objects identified with a lower confidence than the configured confidence will not be saved. (default 50)
    -crop int
        Used with mode=tile. Crop width and height. Uses max_crops to create smaller images from the image and sending each image to the vision model (default: 512) (default 672)
    -debug
        Enable debug mode (default: false)
    -height int
        Resize height (default: 672) (default 672)
    -help
        Show help
    -image string
        Path to the image to process
    -mode string
        'fit' or 'tile'. 'fit' will resize the image to fit the given width and height. 'tile' will resize the image to fit "crop" x "crop" then process the image in 4 tiles with max width and height of "crop". (default "tile")
    -out string
        Path to save the tiled images (default "out")
    -save
        Save cropped images (default: false). For debugging purposes. Images that are saved are not automatically deleted by image-tagger.
    -tags_path string
        Path to the tags file (optional)
    -vision_model string
        Model to use for vision (default: llava:13b) (default "llava:13b")
    -width int
        Resize width (default: 672)

## Building the Project

To build the project, ensure you have Go installed on your system, then run:

```bash
cd cmd/tag
go build
```

## Running the Project

After building, you can run the project using:

```bash
OLLAMA_HOST="http://localhost:11434"
./tag [options]
```

## Debugging the Project

For debugging purposes, you can enable debug logging:

```bash
OLLAMA_HOST="http://localhost:11434"
./tag [options] -debug -save
```

A launch config for debugging is available in the .vscode folder.

## Helpful Commands

Creating link to local image dir:

Windows (CMD) with elevated permissions

```
mklink /d [absolute path]\image-tagger\images  [absolute path]\images
```

Assembling list of images in a dir recursively:

Windows (CMD)

```
for /r %i in (*) do @echo %~fi >> file_list.txt
```

Bash

```
find . -type f -exec realpath {} \; > file_list.txt
```

Merge JSON files in a directory:

```
./test/merge_json.sh ./images
```

Extract tags from the merged JSON file:

```
./test/extract_tags.sh ./images
```

## Ollama Docs

[API Docs](https://github.com/ollama/ollama/blob/main/docs/api.md)
[Go API Examples](https://github.com/ollama/ollama/blob/main/api/examples/README.md)
