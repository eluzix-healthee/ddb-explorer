# DynamoDB TUI Explorer

A terminal user interface (TUI) application for browsing Amazon DynamoDB tables, built with Go and the [tview](https://github.com/rivo/tview) framework.

## Features

- 📋 List all DynamoDB tables with metadata (item count, size, status)
- 🔍 Query tables with partition and sort key conditions
- 📄 Paginated results (15 items per page)
- 🔎 Detailed item inspection with JSON viewer for complex fields
- 🎯 Auto-detection and display of common fields (title, name, description, email)
- ⌨️ Full keyboard navigation
- 🌐 Support for multiple AWS profiles (dev/prod)

## Prerequisites

- Go 1.24 or higher
- AWS credentials configured in `~/.aws/credentials`
- AWS profiles named `dev` and/or `prod` (or customize with `--profile` flag)

## Installation

1. Install dependencies:
```bash
go mod download
```

2. Build the application:
```bash
go build -o ddb-explorer
```

Or use the Makefile:
```bash
make build
```

## Usage

### Running the Application

Run with default profile (prod):
```bash
./ddb-explorer
```

Or simply:
```bash
make
```

Run with a specific profile:
```bash
./ddb-explorer --profile dev
```

### Keyboard Shortcuts

#### Table List View
| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate table list |
| `Enter` | Select table and open query view |
| `q` / `ESC` | Quit application |

#### Query/Scan View
| Key | Action |
|-----|--------|
| `Tab` | Navigate between input fields |
| `Enter` | Execute query |
| `←` / `→` | Switch between Query and Scan tabs |
| `ESC` | Return to table list |

#### Query Results View
| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate results |
| `Enter` | View full item details |
| `Ctrl+N` | Load next page |
| `Ctrl+B` | Go to previous page |
| `ESC` | Return to query view |

#### Item Detail View
| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate item fields |
| `Enter` | View complex field as formatted JSON |
| `ESC` | Return to results view |

#### JSON Viewer
| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll line by line |
| `Space` | Scroll down one page |
| `ESC` | Close JSON viewer |

## Query Conditions

When querying with a sort key, the following conditions are supported:

- `=` - Exact match
- `begins_with` - String starts with value
- `<` - Less than
- `<=` - Less than or equal
- `>` - Greater than
- `>=` - Greater than or equal
- `between` - Between two values (partial support)

## Project Structure

```
ddb-explorer/
├── main.go           # Entry point and UI logic
├── aws/
│   └── dynamodb.go   # AWS DynamoDB client wrapper
├── Makefile          # Build and development tasks
├── go.mod            # Go module definition
├── go.sum            # Go module checksums
└── README.md         # This file
```

## Troubleshooting

### "Failed to connect to AWS"
- Verify your AWS credentials are properly configured
- Check that the profile name matches your credentials file
- Ensure you have network connectivity and proper IAM permissions

### "No tables found"
- Verify the AWS region is correct
- Check that your IAM user/role has `dynamodb:ListTables` permission
- Ensure tables exist in the specified region

### Table appears but can't query
- Verify you have `dynamodb:Query`, `dynamodb:DescribeTable` permissions
- Check that the partition key value is correct and exists

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
