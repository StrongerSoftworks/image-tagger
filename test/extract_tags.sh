#!/bin/bash

# Directory containing JSON files
INPUT_DIR="${1:-./out}"
# Output file
OUTPUT_FILE="${INPUT_DIR}/all_tags.json"

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

# Extract "tags" from each JSON file, flatten the arrays, get unique values, and save to OUTPUT_FILE
jq '[.[].tags[].object] | unique' "$INPUT_DIR"/image_metadata.json > "$OUTPUT_FILE"

# Confirmation message
echo "Unique tags from JSON files in '$INPUT_DIR' have been saved to '$OUTPUT_FILE'."
