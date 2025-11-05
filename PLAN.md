# DDB Viewer TUI Design Plan

## Overview
Create a terminal user interface (TUI) application for browsing Amazon DynamoDB tables using Go and the Bubble Tea framework. The application will allow users to list tables, browse items within tables, and view item details with keyboard navigation.

## Architecture
- **Framework**: Bubble Tea for TUI management
- **AWS SDK**: aws-sdk-go for DynamoDB operations
- **State Management**: Single main model with sub-models for different views
- **Views**: Table list → Table items → Item detail

## Core Components

### 1. Project Structure
```
ddbviewer/
├── main.go           # Entry point
├── models/
│   ├── app.go        # Main application model
│   ├── tablelist.go  # Table list view
│   ├── tableview.go  # Table items view
│   └── itemview.go   # Item detail view
├── aws/
│   └── dynamodb.go   # AWS client wrapper
└── go.mod
```

### 2. Models Design

#### Main App Model
- Current view state (table list, table view, item view)
- AWS client instance
- Navigation history
- Error state

#### Table List Model
- List of table names
- Cursor position
- Loading state
- Pagination support

#### Table Items Model
- Selected table name
- List of items (key-value pairs)
- Cursor position
- Scan/query parameters
- Pagination state

#### Item Detail Model
- Full item data display
- Attribute type handling
- Scrollable view for large items

### 3. Key Features

#### Navigation
- `tab`/`shift+tab`: Switch between views
- `↑`/`↓`: Navigate lists
- `enter`: Select table/item
- `esc`: Go back
- `q`: Quit

#### Data Handling
- Display table names
- Scan tables with pagination
- Show item keys and preview data
- Full item inspection
- Handle different DynamoDB attribute types

#### AWS Integration
- Configurable AWS region/profile
- Credential management
- Error handling for AWS operations

### 4. Implementation Steps

1. **Project Setup**
   - Initialize Go module
   - Add dependencies (bubbletea, aws-sdk-go)
   - Create basic directory structure

2. **AWS Client Setup**
   - Implement DynamoDB client wrapper
   - Handle AWS configuration and credentials
   - Add CLI flag --profile to select AWS profile (default: dev)
   - Support dev/prod profiles
   - Add connection testing

3. **Main Application Model**
   - Define main Bubble Tea model
   - Implement view switching logic
   - Add keyboard event handling

4. **Table List View**
   - List DynamoDB tables
   - Implement pagination
   - Add table selection

5. **Table Items View**
   - Display items from selected table
   - Implement scanning with pagination
   - Show item previews

6. **Item Detail View**
   - Display complete item data
   - Handle complex attribute types
   - Add scrolling for large items

7. **Error Handling & UX**
   - Display AWS errors gracefully
   - Loading states
   - Confirmation dialogs

8. **Testing & Refinement**
   - Unit tests for models
   - Integration testing
   - Performance optimization

## Dependencies
- github.com/charmbracelet/bubbletea
- github.com/aws/aws-sdk-go/aws
- github.com/aws/aws-sdk-go/service/dynamodb

## Success Criteria
- Browse DynamoDB tables in terminal
- View table contents with pagination
- Inspect individual items
- Smooth keyboard navigation
- Proper error handling
- Clean, responsive TUI interface
