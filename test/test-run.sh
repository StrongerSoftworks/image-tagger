#!/bin/bash

# read all image paths from file
for file in ../images/*.jpg; do
    [ -e "$file" ] || continue  # Skip if no files match
    echo "Processing: $file"
    # Run the tag command
    go run ../cmd/tag/main.go -image $file -tags_path tags.json -out out -mode tile -passes 5 -debug
done

# Remove old JSON files
rm out/image_metadata.json
rm out/all_tags.json

# Merge the JSON files
./merge_json.sh out

# Extract tags from the merged JSON file
./extract_tags.sh out


