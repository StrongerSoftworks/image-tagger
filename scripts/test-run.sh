#!/bin/bash

# Run the taglist command
go run ../cmd/taglist/main.go -images_path ../images/file_list.txt -tags_path ../tags.json -out ../images/out -mode fit -summary_model llama3.2:3b -debug

# Remove old files
rm ../images/image_metadata.json
rm ../images/all_tags.json

# Merge the JSON files
./merge_json.sh ../images

# Extract tags from the merged JSON file
./extract_tags.sh ../images


