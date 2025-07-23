#!/bin/bash
#
# File Browser Client
# A shell script to interact with a File Browser instance.
#


# --- Configuration ---

# --- Common Strings & Config ---
FILEBROWSER_URL="${FILEBROWSER_URL:-https://fbcli.app}"
USERNAME="${FILEBROWSER_USERNAME:-fbcliuser}"
PASSWORD="${FILEBROWSER_PASSWORD:-fbcliPass123}"

USER_AGENT="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
ACCEPT_HEADER="Accept: */*"
LANG_HEADER="Accept-Language: en-US,en;q=0.9"
ORIGIN_HEADER="Origin: $FILEBROWSER_URL"
REFERER_ROOT="$FILEBROWSER_URL/"
REFERER_FILES="$FILEBROWSER_URL/files"
CONTENT_TYPE_JSON="Content-Type: application/json"
CONTENT_TYPE_OCTET="Content-Type: application/octet-stream"
CONTENT_TYPE_TEXT="Content-Type: text/plain; charset=UTF-8"

INFO_COLOR='\033[1;34m'
SUCCESS_COLOR='\033[1;32m'
ERROR_COLOR='\033[1;31m'
NC='\033[0m'



# --- Dependency Check ---
REQUIRED_TOOLS=(curl jq python3)
for tool in "${REQUIRED_TOOLS[@]}"; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo -e "${ERROR_COLOR}Error: Required tool '$tool' is not installed. Please install it and try again.${NC}" >&2
    exit 1
  fi
done

# --- URL Encoding Helpers ---
# Usage: url_encode <string>
function url_encode() {
  python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$1"
}
# Usage: url_encode_slash <string> (preserve slashes)
function url_encode_slash() {
  python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe='/'))" "$1"
}

# --- Script Setup ---
set -e # Exit immediately if a command exits with a non-zero status.

# JWT token variable
TOKEN=""

# --- Helper Functions ---


function login() {
  echo -e "${INFO_COLOR}File Browser Client${NC}"
  echo -e "${INFO_COLOR}URL:${NC} $FILEBROWSER_URL"
  echo -e "${INFO_COLOR}Username:${NC} $USERNAME"
  echo -e "${INFO_COLOR}Attempting to log in...${NC}"
  local response_body=$(mktemp)
  local response_code
  response_code=$(curl -s -w "%{http_code}" -o "$response_body" \
    -A "$USER_AGENT" \
    -H "$CONTENT_TYPE_JSON" \
    -H "$ORIGIN_HEADER" \
    -H "$REFERER_ROOT" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
    "$FILEBROWSER_URL/api/login")
  if [ "$response_code" -ne 200 ]; then
    echo "Login failed. Server responded with HTTP $response_code." >&2
    cat "$response_body" >&2
    rm -f "$response_body"
    exit 1
  fi
  TOKEN=$(cat "$response_body")
  if [ -z "$TOKEN" ]; then
    echo "Login failed. No JWT token found in response." >&2
    cat "$response_body"
    rm -f "$response_body"
    exit 1
  fi
  rm -f "$response_body"
  echo -e "${SUCCESS_COLOR}Login successful.${NC}"
}

# --- API Functions ---
function api_get() {
  # Usage: api_get <remote_path> <referer>
  local remote_path="$1"
  local referer="${2:-$REFERER_ROOT}"
  # URL encode remote_path, preserving slashes
  local encoded_remote_path
  encoded_remote_path=$(url_encode_slash "$remote_path")
  local out=$(mktemp)
  local code
  code=$(curl -s -w "%{http_code}" -o "$out" \
    -A "$USER_AGENT" \
    -H "$ACCEPT_HEADER" \
    -H "$LANG_HEADER" \
    -H "$ORIGIN_HEADER" \
    -H "Referer: $referer" \
    --cookie "auth=$TOKEN" \
    "$FILEBROWSER_URL/api/resources$encoded_remote_path")
  if [ "$code" -ne 200 ]; then
    echo -e "${ERROR_COLOR}API error $code for $remote_path${NC}" >&2
    cat "$out" >&2
    rm -f "$out"
    exit 1
  fi
  cat "$out"
  rm -f "$out"
}

function ls() {
  local remote_path="$1"
  # Use robust deduplication and output logic for names with spaces/newlines
  api_get "$remote_path" | jq -r '
    .items[] | [
      (.name | sub("[\\r\\n]+$"; "")) + (if .isDir then "/" else "" end),
      (.modified | sub("T"; " ") | sub("Z$"; "") | split(".") | .[0])
    ] | @tsv' \
    | awk -F'\t' '{
        orig_name=$1;
        norm_name=orig_name;
        sub(/[\r\n]+$/, "", norm_name);
        key=norm_name;
        # Compare modified time lexicographically
        if (!(key in row) || $2 > row[key, "mod"]) {
          row[key, "name"] = orig_name;
          row[key, "mod"] = $2;
        }
        seen[key]=1;
      }
      END {
        for (k in seen) {
          print row[k, "mod"] "\t" row[k, "name"];
        }
      }' \
    | sort -r | cut -f2- | column
}

# Print detailed info like ls -la
function list() {
  local remote_path="$1"
  # Get the data as tab-separated fields to avoid issues with names containing commas or newlines
  local data
  data=$(api_get "$remote_path" | jq -r '
    .items[] | [
      (.name | sub("[\\r\\n]+$"; "")) + (if .isDir then "/" else "" end),
      (.modified | sub("T"; " ") | sub("Z$"; "") | split(".") | .[0]),
      (.size | tostring)
    ] | @tsv')
  # Calculate max name length (capped at 60)
  local maxlen
  maxlen=$(echo "$data" | awk -F'\t' '{if(length($1)>m) m=length($1)} END{print (m>30 && m<60)?m:(m>=60?60:30)}')
  # Print header with dynamic width
  printf "%-${maxlen}s %-20s %-10s\n" "Name" "Modified" "Size"
  printf "%-${maxlen}s %-20s %-10s\n" "$(head -c $maxlen < <(printf '%*s' "$maxlen" | tr ' ' '-'))" "--------------------" "----------"
  # Print rows, deduplicate by normalized name only (remove trailing newlines), keep the most recently modified entry, then sort by modified time descending
  echo "$data" | awk -F'\t' -v w="$maxlen" '
    {
      orig_name=$1;
      norm_name=orig_name;
      sub(/[\r\n]+$/, "", norm_name);
      key=norm_name;
      # Compare modified time (lexicographically, since format is YYYY-MM-DD HH:MM:SS)
      if (!(key in row) || $2 > row[key, "mod"]) {
        row[key, "name"] = orig_name;
        row[key, "mod"] = $2;
        row[key, "size"] = $3;
      }
      seen[key]=1;
    }
    END {
      for (k in seen) {
        print row[k, "mod"] "\t" row[k, "name"] "\t" row[k, "size"];
      }
    }' | sort -r | awk -F'\t' -v w="$maxlen" '{ printf "%-"w"s %-20s %-10s\n", $2, $1, $3 }'
}

function download_file() {
  local remote_path="$1"
  local local_path="$2"
  echo -e "${INFO_COLOR}Downloading '$remote_path' to '$local_path'...${NC}"
  # URL encode remote_path, preserving slashes
  local encoded_remote_path
  encoded_remote_path=$(url_encode_slash "$remote_path")
  if ! curl -s --fail -A "$USER_AGENT" -H "Referer: $FILEBROWSER_URL/" --cookie "auth=$TOKEN" -o "$local_path" "$FILEBROWSER_URL/api/raw$encoded_remote_path"; then
    echo -e "${ERROR_COLOR}Download failed for '$remote_path'.${NC}"
    return 1
  fi
  echo -e "${SUCCESS_COLOR}Download complete.${NC}"
}

function upload_file() {
  local local_path="$1"
  local remote_dir="$2"
  local filename
  filename=$(basename "$local_path")
  echo -e "${INFO_COLOR}Uploading '$local_path' to '$remote_dir/$filename'...${NC}"
  # Ensure remote_dir exists (ignore errors if already exists)
  create_directory "$remote_dir" 2>/dev/null || true
  # URL encode remote_dir and filename, preserving slashes in dir
  local encoded_remote_dir
  encoded_remote_dir=$(url_encode_slash "$remote_dir")
  local encoded_filename
  encoded_filename=$(url_encode "$filename")
  if ! curl -s --fail \
    -A "$USER_AGENT" \
    -H "Accept: */*" \
    -H "Accept-Language: en-US,en;q=0.9" \
    -H "Origin: $FILEBROWSER_URL" \
    -H "Referer: $FILEBROWSER_URL/files" \
    --cookie "auth=$TOKEN" \
    -H "X-Auth: $TOKEN" \
    -H "Content-Type: application/octet-stream" \
    --data-binary "@$local_path" \
    "$FILEBROWSER_URL/api/resources$encoded_remote_dir/$encoded_filename?override=true"; then
    echo -e "${ERROR_COLOR}Upload failed for '$local_path' to '$remote_dir'.${NC}"
    return 1
  fi
  echo -e "${SUCCESS_COLOR}Upload complete.${NC}"
}

function create_directory() {
  local remote_path="$1"
  echo -e "${INFO_COLOR}Creating directory '$remote_path'...${NC}"
  # Ensure trailing slash for directory creation
  local dir_path="$remote_path"
  [[ "$dir_path" != */ ]] && dir_path="$dir_path/"
  # URL encode the directory path, preserving slashes
  local encoded_dir_path
  encoded_dir_path=$(url_encode_slash "$dir_path")
  local mkdir_response
  mkdir_response=$(curl -s -w "%{http_code}" \
    -A "$USER_AGENT" \
    -H "Accept: */*" \
    -H "Accept-Language: en-US,en;q=0.9" \
    -H "Origin: $FILEBROWSER_URL" \
    -H "Referer: $FILEBROWSER_URL/files" \
    --cookie "auth=$TOKEN" \
    -H "X-Auth: $TOKEN" \
    -H "Content-Type: text/plain; charset=UTF-8" \
    -d "" \
    "$FILEBROWSER_URL/api/resources$encoded_dir_path?override=false")
  local mkdir_code=${mkdir_response: -3}
  if [ "$mkdir_code" != "200" ]; then
    echo -e "${ERROR_COLOR}Directory creation failed. Server responded with HTTP $mkdir_code.${NC}"
    echo "$mkdir_response"
    return 1
  fi
  echo -e "${SUCCESS_COLOR}Directory created.${NC}"
}

function delete_resource() {
  local remote_path="$1"
  echo -e "${INFO_COLOR}Deleting '$remote_path'...${NC}"
  # URL encode remote_path but preserve slashes
  local encoded_remote_path
  encoded_remote_path=$(url_encode_slash "$remote_path")
  # Set Referer to parent directory, URL-encoded
  local parent_dir
  parent_dir=$(dirname "$remote_path")
  [[ "$parent_dir" == "." ]] && parent_dir="/"
  local encoded_parent_dir
  encoded_parent_dir=$(url_encode_slash "$parent_dir/")
  local delete_response
  delete_response=$(curl -s -w "%{http_code}" -X DELETE \
    -A "$USER_AGENT" \
    -H "Accept: */*" \
    -H "Accept-Language: en-US,en;q=0.9" \
    -H "Origin: $FILEBROWSER_URL" \
    -H "Referer: $FILEBROWSER_URL/files$encoded_parent_dir" \
    --cookie "auth=$TOKEN" \
    -H "X-Auth: $TOKEN" \
    "$FILEBROWSER_URL/api/resources$encoded_remote_path")
  local delete_code=${delete_response: -3}
  if [ "$delete_code" != "200" ] && [ "$delete_code" != "204" ]; then
    echo -e "${ERROR_COLOR}Delete failed. Server responded with HTTP $delete_code.${NC}"
    echo "$delete_response" | head -c -3
    return 1
  fi
  echo -e "${SUCCESS_COLOR}Deletion complete.${NC}"
}

function rename_resource() {
  local old_path="$1"
  local new_path="$2"
  echo -e "${INFO_COLOR}Renaming '$old_path' to '$new_path'...${NC}"
  # If renaming a directory, ensure trailing slash on both old and new paths
  if [[ "$old_path" == */ ]]; then
    [[ "$new_path" != */ ]] && new_path="$new_path/"
  fi
  # URL encode old_path and destination path, preserving slashes
  local encoded_old_path
  encoded_old_path=$(url_encode_slash "$old_path")
  local encoded_dest
  encoded_dest=$(url_encode_slash "$new_path")
  local rename_response
  rename_response=$(curl -s -w "%{http_code}" \
    -X PATCH \
    -A "$USER_AGENT" \
    -H "Accept: */*" \
    -H "Accept-Language: en-US,en;q=0.9" \
    -H "Origin: $FILEBROWSER_URL" \
    -H "Referer: $FILEBROWSER_URL/files/" \
    --cookie "auth=$TOKEN" \
    -H "X-Auth: $TOKEN" \
    -H "Content-Type: text/plain; charset=UTF-8" \
    -d "" \
    "$FILEBROWSER_URL/api/resources$encoded_old_path?action=rename&destination=$encoded_dest&override=false&rename=false")
  local rename_code=${rename_response: -3}
  if [ "$rename_code" != "200" ]; then
    echo -e "${ERROR_COLOR}Rename failed. Server responded with HTTP $rename_code.${NC}"
    echo "$rename_response"
    return 1
  fi
  echo -e "${SUCCESS_COLOR}Rename complete.${NC}"
}

# --- Usage and Main Script ---

function usage() {
  echo -e "${INFO_COLOR}Usage:${NC} $0 <command> [arguments...]"
  echo ""
  echo "Commands:"
  echo "  list <remote_path>                List files in a remote directory."
  echo "  download <remote_path> <local_path> Download a file."
  echo "  upload <local_path> <remote_dir>    Upload a file to a remote directory."
  echo "  mkdir <remote_path>                 Create a new remote directory."
  echo "  rm <remote_path>                    Delete a remote file or directory."
  echo "  rename <old_path> <new_path>        Rename a remote file or directory."
  echo ""
  echo "All remote paths must start with a '/' (e.g., '/my_folder')."
}

if [ "$#" -eq 0 ]; then
  usage
  exit 1
fi

function validate_remote_path() {
  if [[ ! "$1" == /* ]]; then
    echo "Error: Remote path must be absolute and start with a '/'." >&2
    echo "Incorrect path: '$1'" >&2
    usage
    exit 1
  fi
}

# --- Main Logic ---
login

COMMAND=$1
shift

case $COMMAND in
  ls)
    if [ "$#" -ne 1 ]; then echo "Error: 'ls' requires a <remote_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: ls $1${NC}"
    validate_remote_path "$1"
    ls "$1"
    ;;
  list)
    if [ "$#" -ne 1 ]; then echo "Error: 'list' requires a <remote_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: list $1${NC}"
    validate_remote_path "$1"
    list "$1"
    ;;
  download)
    if [ "$#" -ne 2 ]; then echo "Error: 'download' requires <remote_path> and <local_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: download $1 $2${NC}"
    validate_remote_path "$1"
    download_file "$1" "$2"
    ;;
  upload)
    if [ "$#" -ne 2 ]; then echo "Error: 'upload' requires <local_path> and <remote_dir>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: upload $1 $2${NC}"
    validate_remote_path "$2"
    upload_file "$1" "$2"
    ;;
  mkdir)
    if [ "$#" -ne 1 ]; then echo "Error: 'mkdir' requires a <remote_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: mkdir $1${NC}"
    validate_remote_path "$1"
    create_directory "$1"
    ;;
  rm|delete)
    if [ "$#" -ne 1 ]; then echo "Error: 'rm'/'delete' requires a <remote_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: $COMMAND $1${NC}"
    validate_remote_path "$1"
    delete_resource "$1"
    ;;
  rename|mv)
    if [ "$#" -ne 2 ]; then echo "Error: 'rename'/'mv' requires <old_path> and <new_path>." >&2; usage; exit 1; fi
    echo -e "${INFO_COLOR}Running: $COMMAND $1 $2${NC}"
    validate_remote_path "$1"
    validate_remote_path "$2"
    rename_resource "$1" "$2"
    ;;
  *)
    echo "Error: Unknown command '$COMMAND'" >&2
    usage
    exit 1
    ;;
esac
