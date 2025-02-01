# Image Tagger

Image Tagger is a tool for tagging and getting summaries of the contents of images. It uses vision and text models running on Ollama to generate the tags and summaries. The models it uses can be configured by command arguments.

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

## Usage

```
OLLAMA_HOST="http://localhost:11434"
go run cmd/taglist/main.go -help
```

```
OLLAMA_HOST="http://localhost:11434"
go run cmd/taglist/main.go -images_path images/file_list.txt -tags_path tags.txt -out out -mode fit -debug -save
```

## Arguments:

    -debug
        Enable debug mode (default: false)
    -height int
        Crop or resize height (default: 672) (default 672)
    -help
        Show help
    -images_path string
        Path to the file that contains a list of image file paths
    -max_pixels int
        Max pixels for source image. The source image will be resized if it is larger than configured max pixels (default: 2000000) (default 2000000)
    -mode string
        'fit' or 'tile'. 'fit' will resize the image to fit the given width and height. 'tile' will resize the image to fit the given max pixels then process the image in tiles defined by width and height. (default: fit) (default "fit")
    -out string
        Path to save the tiled images (default "out")
    -save
        Save cropped images (default: false). For debugging purposes. Images that are saved are not automatically deleted by image-tagger.
    -summary_model string
        Model to use for summary (default: mistral:7b) (default "mistral:7b")
    -tags_path string
        Path to the tags file
    -vision_model string
        Model to use for vision (default: llava:13b) (default "llava:13b")
    -width int
        Crop or resize width (default: 672) (default 672)

## Building the Project

To build the project, ensure you have Go installed on your system, then run:

```bash
cd cmd/taglist
go build
```

## Running the Project

After building, you can run the project using:

```bash
OLLAMA_HOST="http://localhost:11434"
./taglist [options]
```

## Debugging the Project

For debugging purposes, you can enable debug logging:

```bash
OLLAMA_HOST="http://localhost:11434"
./taglist [options] -debug -save
```

A launch config for debugging is available in the .vscode folder.

# Helpful Commands

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
./scripts/merge_json.sh ./images
```

Extract tags from the merged JSON file:

```
./scripts/extract_tags.sh ./images
```
