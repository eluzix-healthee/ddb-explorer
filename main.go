package main

import (
	"ddb-explorer/aws"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var profile = flag.String("profile", "dev", "AWS profile to use (dev or prod)")
var showHelp = flag.Bool("help", false, "Show help and usage information")

var tables []aws.TableInfo

// Custom color scheme
var (
	// Background colors
	bgPrimary   = tcell.NewHexColor(0x1a1a1a) // Dark gray
	bgSecondary = tcell.NewHexColor(0x2d2d2d) // Medium gray
	bgAccent    = tcell.NewHexColor(0x404040) // Light gray
	
	// Text colors
	textPrimary   = tcell.NewHexColor(0xe8e8e8) // Light gray
	textSecondary = tcell.NewHexColor(0xb8b8b8) // Medium gray
	textAccent    = tcell.NewHexColor(0xff9500) // Orange (primary)
	
	// Accent colors
	accentOrange = tcell.NewHexColor(0xff9500) // Primary orange
	accentTeal   = tcell.NewHexColor(0x5ac8fa) // Complementary teal
	accentGreen  = tcell.NewHexColor(0x30d158) // Success green
	accentRed    = tcell.NewHexColor(0xff453a) // Error red
	accentYellow = tcell.NewHexColor(0xffd60a) // Warning yellow
)

func applyCustomTheme() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    bgPrimary,
		ContrastBackgroundColor:     accentOrange,
		MoreContrastBackgroundColor: accentTeal,
		BorderColor:                 bgAccent,
		TitleColor:                  accentOrange,
		GraphicsColor:               textPrimary,
		PrimaryTextColor:            textPrimary,
		SecondaryTextColor:          textSecondary,
		TertiaryTextColor:           accentOrange,
		InverseTextColor:            tcell.NewHexColor(0x121212), // Dark text on orange
		ContrastSecondaryTextColor:  textSecondary,
	}
}

func printHelp() {
	fmt.Println(`DynamoDB TUI Explorer - Terminal interface for browsing DynamoDB tables

USAGE:
    ddb-explorer [--profile PROFILE]

OPTIONS:
    --profile    AWS profile to use (default: dev)
    --help       Show this help message

KEYBOARD SHORTCUTS:

Table List View:
    ↑/↓         Navigate table list
    Enter       Select table and open query view
    q/ESC       Quit application

Query/Scan View:
    Tab         Navigate between input fields
    Enter       Execute query
    ←/→         Switch between Query and Scan tabs
    ESC         Return to table list

Query Results View:
    ↑/↓         Navigate results
    Enter       View full item details
    Ctrl+N      Load next page
    Ctrl+B      Go to previous page
    ESC         Return to query view

Item Detail View:
    ↑/↓         Navigate item fields
    Enter       View complex field as formatted JSON
    ESC         Return to results view

JSON Viewer:
    ↑/↓         Scroll line by line
    Space       Scroll down one page
    ESC         Close JSON viewer

EXAMPLES:
    # Run with default (dev) profile
    ./ddb-explorer

    # Run with production profile
    ./ddb-explorer --profile prod

QUERY CONDITIONS:
    =              Exact match
    begins_with    String starts with value
    <, <=, >, >=   Comparison operators
    between        Between two values

For more information, see README.md`)
}

// formatWithCommas formats a number with commas
func formatWithCommas(n int64) string {
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		parts = append([]string{s}, parts...)
	}
	return strings.Join(parts, ",")
}

// formatBytes formats bytes into human-readable size (GB, MB, KB)
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func main() {
	flag.Parse()

	// Show help if requested
	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Apply custom theme before creating any widgets
	applyCustomTheme()

	// Validate profile
	if *profile != "dev" && *profile != "prod" {
		fmt.Printf("Invalid profile: %s. Must be 'dev' or 'prod'\n", *profile)
		os.Exit(1)
	}

	// Create AWS client
	client, err := aws.NewClient(*profile)
	if err != nil {
		fmt.Printf("Failed to create AWS client: %v\n", err)
		os.Exit(1)
	}

	// Test connection
	if err := client.TestConnection(); err != nil {
		fmt.Printf("Failed to connect to AWS: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connected to AWS successfully")

	// Create Tview app
	app := tview.NewApplication()

	// Create pages
	pages := tview.NewPages()

	// Create table
	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)
	
	// Wrap table in flex to add margins and center it
	tableFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).                    // Left margin
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 1, 0, false).                 // Top margin
			AddItem(table, 0, 1, true).                // Table
			AddItem(nil, 1, 0, false), 0, 3, true).   // Bottom margin
		AddItem(nil, 0, 1, false)                     // Right margin

	// Create MOTD-style loading screen
	loadingText := fmt.Sprintf(`
  ____  ____  ____       _____            _                     
 |  _ \|  _ \| __ )     | ____|_  ___ __ | | ___  _ __ ___ _ __ 
 | | | | | | |  _ \ ____|  _| \ \/ / '_ \| |/ _ \| '__/ _ \ '__|
 | |_| | |_| | |_) |____| |___ >  <| |_) | | (_) | | |  __/ |   
 |____/|____/|____/     |_____/_/\_\ .__/|_|\___/|_|  \___|_|   
                                   |_|                           


[orange::b]Loading Tables...[white::-]


[gray]Profile: %s[white::-]
`, *profile)

	loadingView := tview.NewTextView().
		SetText(loadingText).
		SetTextAlign(tview.AlignCenter).
		SetTextColor(accentOrange).
		SetDynamicColors(true)
	loadingView.SetBorder(true).
		SetBorderColor(accentOrange).
		SetTitle(" Welcome ").
		SetTitleColor(accentOrange).
		SetTitleAlign(tview.AlignCenter)

	// Add loading screen as initial page
	pages.AddPage("loading", loadingView, true, true)
	pages.AddPage("tablelist", tableFlex, true, false)

	// Load tables asynchronously
	go func() {
		tableInfos, err := client.ListTables()
		app.QueueUpdateDraw(func() {
			// Switch from loading screen to table list
			pages.SwitchToPage("tablelist")
			
			// Clear any initial state
			table.Clear()

			// Set headers
			headers := []string{"Table Name", "Status", "Item Count", "Size"}
			for col, header := range headers {
				table.SetCell(0, col, tview.NewTableCell(header).
					SetTextColor(tview.Styles.SecondaryTextColor).
					SetSelectable(false).
					SetAlign(tview.AlignCenter))
			}

			if err != nil {
				table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
					SetTextColor(tview.Styles.PrimaryTextColor))
			} else if len(tableInfos) == 0 {
				table.SetCell(1, 0, tview.NewTableCell("No tables found.").
					SetTextColor(tview.Styles.PrimaryTextColor))
			} else {
				tables = tableInfos
				for i, t := range tableInfos {
					table.SetCell(i+1, 0, tview.NewTableCell(t.Name).SetTextColor(tview.Styles.PrimaryTextColor))
					table.SetCell(i+1, 1, tview.NewTableCell(t.Status).SetTextColor(tview.Styles.PrimaryTextColor).SetAlign(tview.AlignCenter))
					table.SetCell(i+1, 2, tview.NewTableCell(formatWithCommas(t.ItemCount)).SetTextColor(tview.Styles.PrimaryTextColor).SetAlign(tview.AlignRight))
					table.SetCell(i+1, 3, tview.NewTableCell(formatBytes(t.SizeBytes)).SetTextColor(tview.Styles.PrimaryTextColor).SetAlign(tview.AlignRight))
				}
				table.ScrollToBeginning()
			}
		})
	}()

	// Set input capture
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC || event.Rune() == 'q' {
			app.Stop()
		} else if event.Key() == tcell.KeyEnter {
			row, _ := table.GetSelection()
			if row > 0 && row <= len(tables) {
				selectedTable := tables[row-1]
				createTableActionPage(pages, app, selectedTable, client)
				pages.SwitchToPage("tableaction")
			}
		}
		return event
	})

	// Set root to pages
	app.SetRoot(pages, true).SetFocus(table)

	// Run app
	if err := app.Run(); err != nil {
		fmt.Printf("Error running app: %v\n", err)
		os.Exit(1)
	}
}

func createTableActionPage(pages *tview.Pages, app *tview.Application, tableInfo aws.TableInfo, client *aws.Client) {
	// Create flex layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Header
	header := tview.NewTextView().
		SetText(fmt.Sprintf("Table: %s (Ctrl+Q: Query | Ctrl+S: Scan)", tableInfo.Name)).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	flex.AddItem(header, 1, 0, false)

	// Form for inputs
	form := tview.NewForm()
	form.SetCancelFunc(func() {
		pages.SwitchToPage("tablelist")
	})
	
	// Apply form styling
	form.SetLabelColor(textSecondary).
		SetFieldBackgroundColor(accentOrange).
		SetFieldTextColor(tcell.NewHexColor(0x121212)).
		SetButtonBackgroundColor(accentOrange).
		SetButtonTextColor(tcell.NewHexColor(0x121212))

	// Tabs flex
	tabsFlex := tview.NewFlex().SetDirection(tview.FlexColumn)

	// Query tab
	queryTab := tview.NewTextView().
		SetText("[ Query ]").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetTextColor(tcell.NewHexColor(0x121212))
	queryTab.SetBackgroundColor(accentOrange)
	tabsFlex.AddItem(queryTab, 0, 1, true)

	// Scan tab
	scanTab := tview.NewTextView().
		SetText("  Scan  ").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetTextColor(textSecondary)
	scanTab.SetBackgroundColor(bgSecondary)
	tabsFlex.AddItem(scanTab, 0, 1, false)

	flex.AddItem(tabsFlex, 1, 0, false)
	flex.AddItem(form, 0, 1, true)

	// Function to update form based on tab
	updateForm := func(tab int) {
		form.Clear(true)
		if tab == 0 { // Query
			if tableInfo.PartitionKey != "" {
				form.AddInputField(fmt.Sprintf("Partition Key (%s)", tableInfo.PartitionKey), "", 20, nil, nil)
			}
			if tableInfo.SortKey != "" {
				form.AddInputField(fmt.Sprintf("Sort Key (%s)", tableInfo.SortKey), "", 20, nil, nil)
				form.AddDropDown("Condition", []string{"=", "begins_with", "<", "<=", ">", ">=", "between"}, 0, nil)
			}
			form.AddButton("Query", func() {
				// Get form values
				var pkValue, skValue, condition string
				if tableInfo.PartitionKey != "" {
					pkValue = form.GetFormItem(0).(*tview.InputField).GetText()
				}
				if tableInfo.SortKey != "" {
					skValue = form.GetFormItem(1).(*tview.InputField).GetText()
					_, condition = form.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
				}

				// Show loading
				loadingModal := tview.NewModal().
					SetText("Querying...").
					SetTextColor(tcell.NewHexColor(0x121212)).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {})
				pages.AddPage("loading", loadingModal, false, true)

				// Perform query async
				go func() {
					var sortKey, sortValue, cond string
					if skValue != "" {
						sortKey = tableInfo.SortKey
						sortValue = skValue
						cond = condition
					}
					result, err := client.Query(tableInfo.Name, tableInfo.PartitionKey, pkValue, sortKey, sortValue, cond, nil)

					app.QueueUpdateDraw(func() {
						pages.RemovePage("loading")
						if err != nil {
							errorModal := tview.NewModal().
								SetText(fmt.Sprintf("Query error: %v", err)).
								AddButtons([]string{"OK"}).
								SetDoneFunc(func(buttonIndex int, buttonLabel string) {
									pages.RemovePage("queryerror")
								})
							pages.AddPage("queryerror", errorModal, true, true)
						} else {
							// Create results table
							resultsTable := tview.NewTable().
					SetBorders(true).
							 SetSelectable(true, false)

							// Detect additional fields to display (title, name, etc.)
							var additionalFields []string
							if len(result.Items) > 0 {
								firstItem := result.Items[0]
								// Common field names to look for
								candidateFields := []string{"title", "Title", "name", "Name", "displayName", "description", "Description", "email", "Email"}
								for _, field := range candidateFields {
									if _, exists := firstItem[field]; exists {
										// Skip if it's already a key field
										if field != tableInfo.PartitionKey && field != tableInfo.SortKey {
											additionalFields = append(additionalFields, field)
											if len(additionalFields) >= 2 {
												break
											}
										}
									}
								}
							}

							// Headers
							headers := []string{tableInfo.PartitionKey}
							if tableInfo.SortKey != "" {
							 headers = append(headers, tableInfo.SortKey)
				}
							headers = append(headers, additionalFields...)
							
							for col, header := range headers {
							 resultsTable.SetCell(0, col, tview.NewTableCell(header).
							 SetTextColor(tview.Styles.SecondaryTextColor).
							 SetSelectable(false).
						SetAlign(tview.AlignCenter))
							}

							// Data
							if len(result.Items) == 0 {
							 resultsTable.SetCell(1, 0, tview.NewTableCell("No items found.").
							 SetTextColor(tview.Styles.PrimaryTextColor))
							} else {
							for i, item := range result.Items {
							col := 0
							resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.PartitionKey])).
							SetTextColor(tview.Styles.PrimaryTextColor))
							col++
							if tableInfo.SortKey != "" {
							resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.SortKey])).
							SetTextColor(tview.Styles.PrimaryTextColor))
							col++
							}
							// Add additional fields
							for _, field := range additionalFields {
								value := ""
								if v, ok := item[field]; ok {
									value = fmt.Sprintf("%v", v)
									// Truncate if too long
									if len(value) > 50 {
										value = value[:47] + "..."
									}
								}
								resultsTable.SetCell(i+1, col, tview.NewTableCell(value).
									SetTextColor(tview.Styles.PrimaryTextColor))
								col++
							}
							}
							 resultsTable.ScrollToBeginning()
				}

				// Add page
				resultsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
				pageHeader := tview.NewTextView().
					SetText(fmt.Sprintf("Query Results for %s - Page 1", tableInfo.Name)).
					SetTextAlign(tview.AlignCenter)
				resultsFlex.AddItem(pageHeader, 1, 0, false)
				resultsFlex.AddItem(resultsTable, 0, 1, true)

				currentPage := 1
				
				// Track pagination history
				type pageState struct {
					items            []map[string]interface{}
					lastEvaluatedKey map[string]interface{}
				}
				pageHistory := []pageState{{items: result.Items, lastEvaluatedKey: result.LastEvaluatedKey}}

				// Function to update results table with new items
				updateResultsTable := func(newResult aws.QueryResult, page int, fields []string) {
					resultsTable.Clear()
					
					// Re-add headers
					headers := []string{tableInfo.PartitionKey}
					if tableInfo.SortKey != "" {
						headers = append(headers, tableInfo.SortKey)
					}
					headers = append(headers, fields...)
					
					for col, header := range headers {
						resultsTable.SetCell(0, col, tview.NewTableCell(header).
							SetTextColor(tview.Styles.SecondaryTextColor).
							SetSelectable(false).
							SetAlign(tview.AlignCenter))
					}

					// Add new data
					if len(newResult.Items) == 0 {
						resultsTable.SetCell(1, 0, tview.NewTableCell("No items found.").
							SetTextColor(tview.Styles.PrimaryTextColor))
					} else {
						for i, item := range newResult.Items {
							col := 0
							resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.PartitionKey])).
								SetTextColor(tview.Styles.PrimaryTextColor))
							col++
							if tableInfo.SortKey != "" {
								resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.SortKey])).
									SetTextColor(tview.Styles.PrimaryTextColor))
								col++
							}
							// Add additional fields
							for _, field := range fields {
								value := ""
								if v, ok := item[field]; ok {
									value = fmt.Sprintf("%v", v)
									// Truncate if too long
									if len(value) > 50 {
										value = value[:47] + "..."
									}
								}
								resultsTable.SetCell(i+1, col, tview.NewTableCell(value).
									SetTextColor(tview.Styles.PrimaryTextColor))
								col++
							}
						}
						resultsTable.ScrollToBeginning()
					}
					
					// Update result reference
					result = newResult
					
					// Update page header
					pageHeader.SetText(fmt.Sprintf("Query Results for %s - Page %d", tableInfo.Name, page))
				}

				// Add navigation buttons
				navFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
				
				btnStyle := tcell.StyleDefault.Background(accentOrange).Foreground(tcell.NewHexColor(0x121212))
				
				loadPrevBtn := tview.NewButton("< Previous (Ctrl+B)").SetSelectedFunc(func() {
					if currentPage > 1 {
						currentPage--
						prevState := pageHistory[currentPage-1]
						updateResultsTable(aws.QueryResult{Items: prevState.items, LastEvaluatedKey: prevState.lastEvaluatedKey}, currentPage, additionalFields)
					}
				})
				loadPrevBtn.SetStyle(btnStyle)
				navFlex.AddItem(loadPrevBtn, 0, 1, false)
				
				var loadNextBtn *tview.Button
				if result.LastEvaluatedKey != nil {
					loadNextBtn = tview.NewButton("Next > (Ctrl+N)").SetSelectedFunc(func() {
						// Load next page
						nextResult, err := client.Query(tableInfo.Name, tableInfo.PartitionKey, pkValue, sortKey, sortValue, cond, result.LastEvaluatedKey)
						if err != nil {
							return
						}
						currentPage++
						
						// Add to history if it's a new page
						if currentPage > len(pageHistory) {
							pageHistory = append(pageHistory, pageState{items: nextResult.Items, lastEvaluatedKey: nextResult.LastEvaluatedKey})
						}
						
						updateResultsTable(nextResult, currentPage, additionalFields)
						
						// If no more pages, remove next button
						if nextResult.LastEvaluatedKey == nil && loadNextBtn != nil {
							navFlex.RemoveItem(loadNextBtn)
						}
					})
					loadNextBtn.SetStyle(btnStyle)
					navFlex.AddItem(loadNextBtn, 0, 1, false)
				}
				
				resultsFlex.AddItem(navFlex, 1, 0, false)
							resultsFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
							if event.Key() == tcell.KeyESC {
							pages.RemovePage("queryresult")
							} else if event.Key() == tcell.KeyCtrlB {
						// Go back to previous page
						if currentPage > 1 {
							currentPage--
							prevState := pageHistory[currentPage-1]
							updateResultsTable(aws.QueryResult{Items: prevState.items, LastEvaluatedKey: prevState.lastEvaluatedKey}, currentPage, additionalFields)
						}
					} else if event.Key() == tcell.KeyCtrlN && result.LastEvaluatedKey != nil {
						// Load next page with Ctrl+N
						nextResult, err := client.Query(tableInfo.Name, tableInfo.PartitionKey, pkValue, sortKey, sortValue, cond, result.LastEvaluatedKey)
						if err != nil {
							return event
						}
						currentPage++
						
						// Add to history if it's a new page
						if currentPage > len(pageHistory) {
							pageHistory = append(pageHistory, pageState{items: nextResult.Items, lastEvaluatedKey: nextResult.LastEvaluatedKey})
						}
						
						updateResultsTable(nextResult, currentPage, additionalFields)
						
						if nextResult.LastEvaluatedKey == nil && loadNextBtn != nil {
							navFlex.RemoveItem(loadNextBtn)
						}
					} else if event.Key() == tcell.KeyEnter {
									row, _ := resultsTable.GetSelection()
									if row > 0 && row <= len(result.Items) {
										item := result.Items[row-1]
										rawItem := result.RawItems[row-1]
										
										// Create item table
										itemTable := tview.NewTable().
											SetBorders(true).
											SetSelectable(true, false)

										// Headers
										itemTable.SetCell(0, 0, tview.NewTableCell("Field").
											SetTextColor(tview.Styles.SecondaryTextColor).
											SetSelectable(false).
											SetAlign(tview.AlignCenter))
										itemTable.SetCell(0, 1, tview.NewTableCell("Value").
											SetTextColor(tview.Styles.SecondaryTextColor).
											SetSelectable(false).
											SetAlign(tview.AlignCenter))
										
										// Function to save item as JSON
										saveItemAsJSON := func() {
											// Generate filename from keys
											pkValue := fmt.Sprintf("%v", rawItem[tableInfo.PartitionKey])
											filename := pkValue
											if tableInfo.SortKey != "" {
												skValue := fmt.Sprintf("%v", rawItem[tableInfo.SortKey])
												filename = fmt.Sprintf("%s_%s", pkValue, skValue)
											}
											// Clean filename (remove special characters)
											filename = strings.ReplaceAll(filename, "/", "_")
											filename = strings.ReplaceAll(filename, " ", "_")
											filename = strings.ReplaceAll(filename, ":", "_")
											filename += ".json"
											
											// Marshal to JSON
											jsonBytes, err := json.MarshalIndent(rawItem, "", "    ")
											if err != nil {
												// Show error
												errorModal := tview.NewModal().
													SetText(fmt.Sprintf("Error saving JSON: %v", err)).
													AddButtons([]string{"OK"}).
													SetDoneFunc(func(buttonIndex int, buttonLabel string) {
														pages.RemovePage("saveerror")
													})
												pages.AddPage("saveerror", errorModal, true, true)
												return
											}
											
											// Write to file
											err = os.WriteFile(filename, jsonBytes, 0644)
											if err != nil {
												// Show error
												errorModal := tview.NewModal().
													SetText(fmt.Sprintf("Error writing file: %v", err)).
													AddButtons([]string{"OK"}).
													SetDoneFunc(func(buttonIndex int, buttonLabel string) {
														pages.RemovePage("saveerror")
													})
												pages.AddPage("saveerror", errorModal, true, true)
												return
											}
											
											// Show success
											successModal := tview.NewModal().
												SetText(fmt.Sprintf("Saved to: %s", filename)).
												AddButtons([]string{"OK"}).
												SetDoneFunc(func(buttonIndex int, buttonLabel string) {
													pages.RemovePage("savesuccess")
												})
											pages.AddPage("savesuccess", successModal, true, true)
										}

										// Data: schema fields first
										i := 1
										// Schema fields
										for _, sf := range tableInfo.SchemaFields {
											if v, ok := item[sf]; ok {
												displayValue := fmt.Sprintf("%v", v)
												itemTable.SetCell(i, 0, tview.NewTableCell(sf).
													SetTextColor(tview.Styles.SecondaryTextColor).
													SetSelectable(true))
												itemTable.SetCell(i, 1, tview.NewTableCell(displayValue).
													SetTextColor(tview.Styles.SecondaryTextColor).
													SetSelectable(true))
												delete(item, sf)
												i++
											}
										}
										// Other fields
										for k, v := range item {
											displayValue := fmt.Sprintf("%v", v)
											itemTable.SetCell(i, 0, tview.NewTableCell(k).
												SetTextColor(tview.Styles.PrimaryTextColor).
												SetSelectable(true))
											itemTable.SetCell(i, 1, tview.NewTableCell(displayValue).
												SetTextColor(tview.Styles.PrimaryTextColor).
												SetSelectable(true))
											i++
										}
										itemTable.ScrollToBeginning()

										// Create flex for the table
										itemFlex := tview.NewFlex().SetDirection(tview.FlexRow)
										itemFlex.AddItem(tview.NewTextView().SetText("Full Item (Ctrl+D: download as JSON)").SetTextAlign(tview.AlignCenter), 1, 0, false)
										itemFlex.AddItem(itemTable, 0, 1, true)
										itemFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
											if event.Key() == tcell.KeyESC {
												pages.RemovePage("fullitem")
											} else if event.Key() == tcell.KeyCtrlD {
												saveItemAsJSON()
												return nil
											} else if event.Key() == tcell.KeyEnter {
												row, _ := itemTable.GetSelection()
												if row > 0 {
													fieldCell := itemTable.GetCell(row, 0)
													fieldName := fieldCell.Text
													if v, ok := rawItem[fieldName]; ok {
														// Check if it's a complex type (map or slice)
														switch v.(type) {
														case map[string]interface{}, []interface{}:
															// Format as JSON
															jsonBytes, err := json.MarshalIndent(v, "", "    ")
															if err != nil {
																jsonBytes = []byte(fmt.Sprintf("Error formatting JSON: %v", err))
															}
															jsonView := tview.NewTextView().
																SetText(string(jsonBytes)).
																SetTextAlign(tview.AlignLeft).
																SetDynamicColors(true).
																SetScrollable(true).
																SetWrap(true)
															
															jsonFlex := tview.NewFlex().SetDirection(tview.FlexRow)
															jsonFlex.AddItem(tview.NewTextView().SetText(fmt.Sprintf("JSON View - %s (Space: page down, ESC: close)", fieldName)).SetTextAlign(tview.AlignCenter), 1, 0, false)
															jsonFlex.AddItem(jsonView, 0, 1, true)
															
															jsonView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
																if event.Key() == tcell.KeyESC {
																	pages.RemovePage("jsonview")
																	return nil
																} else if event.Rune() == ' ' {
																	// Scroll down by page
																	row, col := jsonView.GetScrollOffset()
																	_, _, _, height := jsonView.GetInnerRect()
																	jsonView.ScrollTo(row+height-1, col)
																	return nil
																}
																return event
															})
															
															pages.AddPage("jsonview", jsonFlex, true, true)
															app.SetFocus(jsonView)
														}
													}
												}
											}
											return event
										})

										pages.AddPage("fullitem", itemFlex, true, true)
									}
								}
								return event
							})
							pages.AddPage("queryresult", resultsFlex, true, true)
						}
					})
				}()
			})
			
			// Set focus to form itself to enable Tab navigation
			app.SetFocus(form)
		} else { // Scan
			form.AddButton(fmt.Sprintf("Scan %s", tableInfo.Name), func() {
				// Show loading modal
				loadingModal := tview.NewModal().
					SetText("Scanning...").
					SetTextColor(tcell.NewHexColor(0x121212))
				pages.AddPage("loadingscan", loadingModal, true, true)

				// Perform scan async
				go func() {
					result, err := client.Scan(tableInfo.Name, nil)

					app.QueueUpdateDraw(func() {
						pages.RemovePage("loadingscan")
						pages.RemovePage("scanresult") // Remove any existing scan results
						if err != nil {
							errorModal := tview.NewModal().
								SetText(fmt.Sprintf("Scan error: %v", err)).
								AddButtons([]string{"OK"}).
								SetDoneFunc(func(buttonIndex int, buttonLabel string) {
									pages.RemovePage("scanerror")
								})
							pages.AddPage("scanerror", errorModal, true, true)
						} else {
							// Create results table
							resultsTable := tview.NewTable().
								SetBorders(true).
								SetSelectable(true, false)

							// Detect additional fields to display (title, name, etc.)
							var additionalFields []string
							if len(result.Items) > 0 {
								firstItem := result.Items[0]
								// Common field names to look for
								candidateFields := []string{"title", "Title", "name", "Name", "displayName", "description", "Description", "email", "Email"}
								for _, field := range candidateFields {
									if _, exists := firstItem[field]; exists {
										// Skip if it's already a key field
										if field != tableInfo.PartitionKey && field != tableInfo.SortKey {
											additionalFields = append(additionalFields, field)
											if len(additionalFields) >= 2 {
												break
											}
										}
									}
								}
							}

							// Headers
							headers := []string{tableInfo.PartitionKey}
							if tableInfo.SortKey != "" {
								headers = append(headers, tableInfo.SortKey)
							}
							headers = append(headers, additionalFields...)
							
							for col, header := range headers {
								resultsTable.SetCell(0, col, tview.NewTableCell(header).
									SetTextColor(tview.Styles.SecondaryTextColor).
									SetSelectable(false).
									SetAlign(tview.AlignCenter))
							}

							// Data
							if len(result.Items) == 0 {
								resultsTable.SetCell(1, 0, tview.NewTableCell("No items found.").
									SetTextColor(tview.Styles.PrimaryTextColor))
							} else {
								for i, item := range result.Items {
									col := 0
									resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.PartitionKey])).
										SetTextColor(tview.Styles.PrimaryTextColor))
									col++
									if tableInfo.SortKey != "" {
										resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.SortKey])).
											SetTextColor(tview.Styles.PrimaryTextColor))
										col++
									}
									// Add additional fields
									for _, field := range additionalFields {
										value := ""
										if v, ok := item[field]; ok {
											value = fmt.Sprintf("%v", v)
											// Truncate if too long
											if len(value) > 50 {
												value = value[:47] + "..."
											}
										}
										resultsTable.SetCell(i+1, col, tview.NewTableCell(value).
											SetTextColor(tview.Styles.PrimaryTextColor))
										col++
									}
								}
								resultsTable.ScrollToBeginning()
							}

							// Add page
							resultsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
							pageHeader := tview.NewTextView().
								SetText(fmt.Sprintf("Scan Results for %s - Page 1", tableInfo.Name)).
								SetTextAlign(tview.AlignCenter)
							resultsFlex.AddItem(pageHeader, 1, 0, false)
							resultsFlex.AddItem(resultsTable, 0, 1, true)

							currentPage := 1
							
							// Track pagination history
							type pageState struct {
								items            []map[string]interface{}
								rawItems         []map[string]interface{}
								lastEvaluatedKey map[string]interface{}
							}
							pageHistory := []pageState{{items: result.Items, rawItems: result.RawItems, lastEvaluatedKey: result.LastEvaluatedKey}}

							// Function to update results table with new items
							updateResultsTable := func(newResult aws.QueryResult, page int, fields []string) {
								resultsTable.Clear()
								
								// Re-add headers
								headers := []string{tableInfo.PartitionKey}
								if tableInfo.SortKey != "" {
									headers = append(headers, tableInfo.SortKey)
								}
								headers = append(headers, fields...)
								
								for col, header := range headers {
									resultsTable.SetCell(0, col, tview.NewTableCell(header).
										SetTextColor(tview.Styles.SecondaryTextColor).
										SetSelectable(false).
										SetAlign(tview.AlignCenter))
								}

								// Add new data
								if len(newResult.Items) == 0 {
									resultsTable.SetCell(1, 0, tview.NewTableCell("No items found.").
										SetTextColor(tview.Styles.PrimaryTextColor))
								} else {
									for i, item := range newResult.Items {
										col := 0
										resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.PartitionKey])).
											SetTextColor(tview.Styles.PrimaryTextColor))
										col++
										if tableInfo.SortKey != "" {
											resultsTable.SetCell(i+1, col, tview.NewTableCell(fmt.Sprintf("%v", item[tableInfo.SortKey])).
												SetTextColor(tview.Styles.PrimaryTextColor))
											col++
										}
										// Add additional fields
										for _, field := range fields {
											value := ""
											if v, ok := item[field]; ok {
												value = fmt.Sprintf("%v", v)
												// Truncate if too long
												if len(value) > 50 {
													value = value[:47] + "..."
												}
											}
											resultsTable.SetCell(i+1, col, tview.NewTableCell(value).
												SetTextColor(tview.Styles.PrimaryTextColor))
											col++
										}
									}
									resultsTable.ScrollToBeginning()
								}
								
								// Update result reference
								result = newResult
								
								// Update page header
								pageHeader.SetText(fmt.Sprintf("Scan Results for %s - Page %d", tableInfo.Name, page))
							}

							// Add navigation buttons
							navFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
							
							btnStyle := tcell.StyleDefault.Background(accentOrange).Foreground(tcell.NewHexColor(0x121212))
							
							loadPrevBtn := tview.NewButton("< Previous (Ctrl+B)").SetSelectedFunc(func() {
								if currentPage > 1 {
									currentPage--
									prevState := pageHistory[currentPage-1]
									updateResultsTable(aws.QueryResult{Items: prevState.items, RawItems: prevState.rawItems, LastEvaluatedKey: prevState.lastEvaluatedKey}, currentPage, additionalFields)
								}
							})
							loadPrevBtn.SetStyle(btnStyle)
							navFlex.AddItem(loadPrevBtn, 0, 1, false)
							
							var loadNextBtn *tview.Button
							if result.LastEvaluatedKey != nil {
								loadNextBtn = tview.NewButton("Next > (Ctrl+N)").SetSelectedFunc(func() {
									// Load next page
									nextResult, err := client.Scan(tableInfo.Name, result.LastEvaluatedKey)
									if err != nil {
										return
									}
									currentPage++
									
									// Add to history if it's a new page
									if currentPage > len(pageHistory) {
										pageHistory = append(pageHistory, pageState{items: nextResult.Items, rawItems: nextResult.RawItems, lastEvaluatedKey: nextResult.LastEvaluatedKey})
									}
									
									updateResultsTable(nextResult, currentPage, additionalFields)
									
									// If no more pages, remove next button
									if nextResult.LastEvaluatedKey == nil && loadNextBtn != nil {
										navFlex.RemoveItem(loadNextBtn)
									}
								})
								loadNextBtn.SetStyle(btnStyle)
								navFlex.AddItem(loadNextBtn, 0, 1, false)
							}
							
							resultsFlex.AddItem(navFlex, 1, 0, false)

							resultsFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
								if event.Key() == tcell.KeyESC {
									pages.RemovePage("scanresult")
								} else if event.Key() == tcell.KeyCtrlB {
									// Go back to previous page
									if currentPage > 1 {
										currentPage--
										prevState := pageHistory[currentPage-1]
										updateResultsTable(aws.QueryResult{Items: prevState.items, RawItems: prevState.rawItems, LastEvaluatedKey: prevState.lastEvaluatedKey}, currentPage, additionalFields)
									}
								} else if event.Key() == tcell.KeyCtrlN && result.LastEvaluatedKey != nil {
									// Load next page with Ctrl+N
									nextResult, err := client.Scan(tableInfo.Name, result.LastEvaluatedKey)
									if err != nil {
										return event
									}
									currentPage++
									
									// Add to history if it's a new page
									if currentPage > len(pageHistory) {
										pageHistory = append(pageHistory, pageState{items: nextResult.Items, rawItems: nextResult.RawItems, lastEvaluatedKey: nextResult.LastEvaluatedKey})
									}
									
									updateResultsTable(nextResult, currentPage, additionalFields)
									
									if nextResult.LastEvaluatedKey == nil && loadNextBtn != nil {
										navFlex.RemoveItem(loadNextBtn)
									}
								} else if event.Key() == tcell.KeyEnter {
									row, _ := resultsTable.GetSelection()
									if row > 0 && row <= len(result.Items) {
										item := result.Items[row-1]
										rawItem := result.RawItems[row-1]
										
										// Create item table (reuse same logic as query)
										itemTable := tview.NewTable().
											SetBorders(true).
											SetSelectable(true, false)

										// Headers
										itemTable.SetCell(0, 0, tview.NewTableCell("Field").
											SetTextColor(tview.Styles.SecondaryTextColor).
											SetSelectable(false).
											SetAlign(tview.AlignCenter))
										itemTable.SetCell(0, 1, tview.NewTableCell("Value").
											SetTextColor(tview.Styles.SecondaryTextColor).
											SetSelectable(false).
											SetAlign(tview.AlignCenter))
										
										// Function to save item as JSON
										saveItemAsJSON := func() {
											// Generate filename from keys
											pkValue := fmt.Sprintf("%v", rawItem[tableInfo.PartitionKey])
											filename := pkValue
											if tableInfo.SortKey != "" {
												skValue := fmt.Sprintf("%v", rawItem[tableInfo.SortKey])
												filename = fmt.Sprintf("%s_%s", pkValue, skValue)
											}
											// Clean filename (remove special characters)
											filename = strings.ReplaceAll(filename, "/", "_")
											filename = strings.ReplaceAll(filename, " ", "_")
											filename = strings.ReplaceAll(filename, ":", "_")
											filename += ".json"
											
											// Marshal to JSON
											jsonBytes, err := json.MarshalIndent(rawItem, "", "    ")
											if err != nil {
												// Show error
												errorModal := tview.NewModal().
													SetText(fmt.Sprintf("Error saving JSON: %v", err)).
													AddButtons([]string{"OK"}).
													SetDoneFunc(func(buttonIndex int, buttonLabel string) {
														pages.RemovePage("saveerror")
													})
												pages.AddPage("saveerror", errorModal, true, true)
												return
											}
											
											// Write to file
											err = os.WriteFile(filename, jsonBytes, 0644)
											if err != nil {
												// Show error
												errorModal := tview.NewModal().
													SetText(fmt.Sprintf("Error writing file: %v", err)).
													AddButtons([]string{"OK"}).
													SetDoneFunc(func(buttonIndex int, buttonLabel string) {
														pages.RemovePage("saveerror")
													})
												pages.AddPage("saveerror", errorModal, true, true)
												return
											}
											
											// Show success
											successModal := tview.NewModal().
												SetText(fmt.Sprintf("Saved to: %s", filename)).
												AddButtons([]string{"OK"}).
												SetDoneFunc(func(buttonIndex int, buttonLabel string) {
													pages.RemovePage("savesuccess")
												})
											pages.AddPage("savesuccess", successModal, true, true)
										}

										// Data: schema fields first
										i := 1
										// Schema fields
										for _, sf := range tableInfo.SchemaFields {
											if v, ok := item[sf]; ok {
												displayValue := fmt.Sprintf("%v", v)
												itemTable.SetCell(i, 0, tview.NewTableCell(sf).
													SetTextColor(tview.Styles.SecondaryTextColor).
													SetSelectable(true))
												itemTable.SetCell(i, 1, tview.NewTableCell(displayValue).
													SetTextColor(tview.Styles.SecondaryTextColor).
													SetSelectable(true))
												delete(item, sf)
												i++
											}
										}
										// Other fields
										for k, v := range item {
											displayValue := fmt.Sprintf("%v", v)
											itemTable.SetCell(i, 0, tview.NewTableCell(k).
												SetTextColor(tview.Styles.PrimaryTextColor).
												SetSelectable(true))
											itemTable.SetCell(i, 1, tview.NewTableCell(displayValue).
												SetTextColor(tview.Styles.PrimaryTextColor).
												SetSelectable(true))
											i++
										}
										itemTable.ScrollToBeginning()

										// Create flex for the table
										itemFlex := tview.NewFlex().SetDirection(tview.FlexRow)
										itemFlex.AddItem(tview.NewTextView().SetText("Full Item (Ctrl+D: download as JSON)").SetTextAlign(tview.AlignCenter), 1, 0, false)
										itemFlex.AddItem(itemTable, 0, 1, true)
										itemFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
											if event.Key() == tcell.KeyESC {
												pages.RemovePage("fullitem")
											} else if event.Key() == tcell.KeyCtrlD {
												saveItemAsJSON()
												return nil
											} else if event.Key() == tcell.KeyEnter {
												row, _ := itemTable.GetSelection()
												if row > 0 {
													fieldCell := itemTable.GetCell(row, 0)
													fieldName := fieldCell.Text
													if v, ok := rawItem[fieldName]; ok {
														// Check if it's a complex type (map or slice)
														switch v.(type) {
														case map[string]interface{}, []interface{}:
															// Format as JSON
															jsonBytes, err := json.MarshalIndent(v, "", "    ")
															if err != nil {
																jsonBytes = []byte(fmt.Sprintf("Error formatting JSON: %v", err))
															}
															jsonView := tview.NewTextView().
																SetText(string(jsonBytes)).
																SetTextAlign(tview.AlignLeft).
																SetDynamicColors(true).
																SetScrollable(true).
																SetWrap(true)
															
															jsonFlex := tview.NewFlex().SetDirection(tview.FlexRow)
															jsonFlex.AddItem(tview.NewTextView().SetText(fmt.Sprintf("JSON View - %s (Space: page down, ESC: close)", fieldName)).SetTextAlign(tview.AlignCenter), 1, 0, false)
															jsonFlex.AddItem(jsonView, 0, 1, true)
															
															jsonView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
																if event.Key() == tcell.KeyESC {
																	pages.RemovePage("jsonview")
																	return nil
																} else if event.Rune() == ' ' {
																	// Scroll down by page
																	row, col := jsonView.GetScrollOffset()
																	_, _, _, height := jsonView.GetInnerRect()
																	jsonView.ScrollTo(row+height-1, col)
																	return nil
																}
																return event
															})
															
															pages.AddPage("jsonview", jsonFlex, true, true)
															app.SetFocus(jsonView)
														}
													}
												}
											}
											return event
										})

										pages.AddPage("fullitem", itemFlex, true, true)
									}
								}
								return event
							})
							pages.AddPage("scanresult", resultsFlex, true, true)
							app.SetFocus(resultsTable)
						}
					})
				}()
			})
			
			// Set focus to form itself
			app.SetFocus(form)
		}
	}

	// Initial form
	updateForm(0)

	// Set input capture for tab switching
	currentTab := 0 // 0: Query, 1: Scan
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC {
			pages.SwitchToPage("tablelist")
		} else if event.Key() == tcell.KeyCtrlQ {
			// Switch to Query tab
			if currentTab != 0 {
				currentTab = 0
				updateForm(currentTab)
				queryTab.SetTextColor(tcell.NewHexColor(0x121212))
				queryTab.SetBackgroundColor(accentOrange)
				scanTab.SetTextColor(textSecondary)
				scanTab.SetBackgroundColor(bgSecondary)
			}
			return nil
		} else if event.Key() == tcell.KeyCtrlS {
			// Switch to Scan tab
			if currentTab != 1 {
				currentTab = 1
				updateForm(currentTab)
				queryTab.SetTextColor(textSecondary)
				queryTab.SetBackgroundColor(bgSecondary)
				scanTab.SetTextColor(tcell.NewHexColor(0x121212))
				scanTab.SetBackgroundColor(accentOrange)
			}
			return nil
		} else if event.Key() == tcell.KeyRight || event.Key() == tcell.KeyLeft {
			currentTab = 1 - currentTab
			updateForm(currentTab)
			if currentTab == 0 {
				queryTab.SetTextColor(tcell.NewHexColor(0x121212))
				queryTab.SetBackgroundColor(accentOrange)
				scanTab.SetTextColor(textSecondary)
				scanTab.SetBackgroundColor(bgSecondary)
			} else {
				queryTab.SetTextColor(textSecondary)
				queryTab.SetBackgroundColor(bgSecondary)
				scanTab.SetTextColor(tcell.NewHexColor(0x121212))
				scanTab.SetBackgroundColor(accentOrange)
			}
		}
		return event
	})

	// Add page
	pages.AddPage("tableaction", flex, true, false)
}
