#!/bin/bash

# Output file
INPUT_DIR="${1:-./out}"
OUTPUT_FILE="${INPUT_DIR}/image_metadata.json"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install jq to use this script."
    exit 1
fi

# Check if the input directory exists
if [ ! -d "$INPUT_DIR" ]; then
    echo "Error: Directory '$INPUT_DIR' does not exist."
    exit 1
fi

# Combine all JSON files into an array and save to OUTPUT_FILE
jq -s '.' "$INPUT_DIR"/*.json > "$OUTPUT_FILE"

# Confirmation message
echo "All JSON files in '$INPUT_DIR' have been combined into '$OUTPUT_FILE'."
