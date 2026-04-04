# API Reference

## File Commands

### ls [path]
List directory contents. Defaults to root `/` if no path given.

### cat <path>
Display file contents by reassembling chunks in order.

### stat <path>
Show file metadata: size, modification time, type.

### find <path> -name <pattern>
Search for files matching a glob pattern.

## Search Commands

### grep <pattern> <path>
Search for text pattern in files under a path.

### search <query> <path>
Semantic search using vector similarity.

### ingest <local-dir> <vkfs-path>
Import local files into the virtual filesystem.
