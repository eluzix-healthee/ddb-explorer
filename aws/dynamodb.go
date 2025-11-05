package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Client wraps the DynamoDB client
type Client struct {
	svc *dynamodb.Client
}

// NewClient creates a new DynamoDB client with the given profile
func NewClient(profile string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion("us-east-1"), // TODO: make configurable
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config with profile %s: %w", profile, err)
	}

	svc := dynamodb.NewFromConfig(cfg)
	return &Client{svc: svc}, nil
}

// TestConnection tests the connection by listing tables
func (c *Client) TestConnection() error {
	_, err := c.svc.ListTables(context.TODO(), &dynamodb.ListTablesInput{})
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	return nil
}

// TableInfo holds table metadata
type TableInfo struct {
	Name         string
	Status       string
	ItemCount    int64
	SizeBytes    int64
	PartitionKey string
	SortKey      string
	SchemaFields []string
}

// ListTables returns a list of table info
func (c *Client) ListTables() ([]TableInfo, error) {
	result, err := c.svc.ListTables(context.TODO(), &dynamodb.ListTablesInput{})
	if err != nil {
		return nil, err
	}

	var tables []TableInfo
	for _, name := range result.TableNames {
		info, err := c.getTableInfo(name)
		if err != nil {
			// Skip tables with errors, or return partial
			continue
		}
		tables = append(tables, info)
	}

	// Sort by ItemCount descending
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].ItemCount > tables[j].ItemCount
	})

	return tables, nil
}

// QueryResult holds query results
type QueryResult struct {
	Items             []map[string]interface{}
	RawItems          []map[string]interface{} // Structured data for JSON viewing
	LastEvaluatedKey map[string]interface{}
}

// Query executes a query on the table (first batch only)

// attributeValueToInterface converts a DynamoDB attribute value to Go native types
func attributeValueToInterface(v types.AttributeValue) interface{} {
	switch val := v.(type) {
	case *types.AttributeValueMemberS:
		return val.Value
	case *types.AttributeValueMemberN:
		// Try to parse as int, fallback to string
		if i, err := strconv.ParseInt(val.Value, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(val.Value, 64); err == nil {
			return f
		}
		return val.Value
	case *types.AttributeValueMemberBOOL:
		return val.Value
	case *types.AttributeValueMemberNULL:
		return nil
	case *types.AttributeValueMemberL:
		list := make([]interface{}, len(val.Value))
		for i, av := range val.Value {
			list[i] = attributeValueToInterface(av)
		}
		return list
	case *types.AttributeValueMemberM:
		m := make(map[string]interface{})
		for k, av := range val.Value {
			m[k] = attributeValueToInterface(av)
		}
		return m
	case *types.AttributeValueMemberSS:
		return val.Value
	case *types.AttributeValueMemberNS:
		return val.Value
	case *types.AttributeValueMemberBS:
		strs := make([]string, len(val.Value))
		for i, b := range val.Value {
			strs[i] = fmt.Sprintf("<binary: %d bytes>", len(b))
		}
		return strs
	case *types.AttributeValueMemberB:
		return fmt.Sprintf("<binary: %d bytes>", len(val.Value))
	default:
		return "unknown"
	}
}

// formatAttributeValue formats a DynamoDB attribute value
func formatAttributeValue(v types.AttributeValue) string {
	switch val := v.(type) {
	case *types.AttributeValueMemberS:
		return val.Value
	case *types.AttributeValueMemberN:
		return val.Value
	case *types.AttributeValueMemberB:
		return fmt.Sprintf("<binary: %d bytes>", len(val.Value))
	case *types.AttributeValueMemberSS:
		return fmt.Sprintf("[%s]", strings.Join(val.Value, ", "))
	case *types.AttributeValueMemberNS:
		return fmt.Sprintf("[%s]", strings.Join(val.Value, ", "))
	case *types.AttributeValueMemberBS:
		strs := make([]string, len(val.Value))
		for i, b := range val.Value {
			strs[i] = fmt.Sprintf("<binary: %d bytes>", len(b))
		}
		return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
	case *types.AttributeValueMemberL:
		strs := make([]string, len(val.Value))
		for i, av := range val.Value {
			strs[i] = formatAttributeValue(av)
		}
		return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
	case *types.AttributeValueMemberM:
		var pairs []string
		for k, av := range val.Value {
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, formatAttributeValue(av)))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case *types.AttributeValueMemberNULL:
		return "null"
	case *types.AttributeValueMemberBOOL:
		return fmt.Sprintf("%t", val.Value)
	default:
		return "unknown"
	}
}

func (c *Client) Query(tableName, partitionKey, partitionValue, sortKey, sortValue, condition string, exclusiveStartKey map[string]interface{}) (QueryResult, error) {
	limit := int32(15) // Load batch of 15 items
	input := &dynamodb.QueryInput{
		TableName: &tableName,
		Limit:     &limit,
		KeyConditionExpression: aws.String(fmt.Sprintf("#pk = :pk")),
		ExpressionAttributeNames: map[string]string{
			"#pk": partitionKey,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: partitionValue},
		},
	}

	if exclusiveStartKey != nil {
		// Convert map to AttributeValue map
		exclKey := make(map[string]types.AttributeValue)
		for k, v := range exclusiveStartKey {
			switch val := v.(type) {
			case string:
				exclKey[k] = &types.AttributeValueMemberS{Value: val}
			case int64:
				exclKey[k] = &types.AttributeValueMemberN{Value: strconv.FormatInt(val, 10)}
			// Add more types if needed
			}
		}
		input.ExclusiveStartKey = exclKey
	}

	if sortKey != "" && sortValue != "" {
		// Add sort key condition
		switch condition {
		case "=":
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk = :sk")
		case "begins_with":
			input.KeyConditionExpression = aws.String("#pk = :pk AND begins_with(#sk, :sk)")
		case "<":
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk < :sk")
		case "<=":
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk <= :sk")
		case ">":
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk > :sk")
		case ">=":
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk >= :sk")
		case "between":
			// For between, need two values, but for now assume single
			input.KeyConditionExpression = aws.String("#pk = :pk AND #sk BETWEEN :sk AND :sk2")
			// TODO: handle between properly
		}
		input.ExpressionAttributeNames["#sk"] = sortKey
		input.ExpressionAttributeValues[":sk"] = &types.AttributeValueMemberS{Value: sortValue}
	}

	result, err := c.svc.Query(context.TODO(), input)
	if err != nil {
		return QueryResult{}, err
	}

	// Convert items (formatted strings for display)
	items := make([]map[string]interface{}, len(result.Items))
	rawItems := make([]map[string]interface{}, len(result.Items))
	for i, item := range result.Items {
		items[i] = make(map[string]interface{})
		rawItems[i] = make(map[string]interface{})
		for k, v := range item {
			items[i][k] = formatAttributeValue(v)
			rawItems[i][k] = attributeValueToInterface(v)
		}
	}

	// Convert LastEvaluatedKey
	var lastKey map[string]interface{}
	if result.LastEvaluatedKey != nil {
		lastKey = make(map[string]interface{})
		for k, v := range result.LastEvaluatedKey {
			lastKey[k] = formatAttributeValue(v)
		}
	}

	return QueryResult{Items: items, RawItems: rawItems, LastEvaluatedKey: lastKey}, nil
}

// Scan executes a scan on the table
func (c *Client) Scan(tableName string, exclusiveStartKey map[string]interface{}) (QueryResult, error) {
	limit := int32(15) // Load batch of 15 items
	input := &dynamodb.ScanInput{
		TableName: &tableName,
		Limit:     &limit,
	}

	if exclusiveStartKey != nil {
		// Convert map to AttributeValue map
		exclKey := make(map[string]types.AttributeValue)
		for k, v := range exclusiveStartKey {
			switch val := v.(type) {
			case string:
				exclKey[k] = &types.AttributeValueMemberS{Value: val}
			case int64:
				exclKey[k] = &types.AttributeValueMemberN{Value: strconv.FormatInt(val, 10)}
			// Add more types if needed
			}
		}
		input.ExclusiveStartKey = exclKey
	}

	result, err := c.svc.Scan(context.TODO(), input)
	if err != nil {
		return QueryResult{}, err
	}

	// Convert items (formatted strings for display)
	items := make([]map[string]interface{}, len(result.Items))
	rawItems := make([]map[string]interface{}, len(result.Items))
	for i, item := range result.Items {
		items[i] = make(map[string]interface{})
		rawItems[i] = make(map[string]interface{})
		for k, v := range item {
			items[i][k] = formatAttributeValue(v)
			rawItems[i][k] = attributeValueToInterface(v)
		}
	}

	// Convert LastEvaluatedKey
	var lastKey map[string]interface{}
	if result.LastEvaluatedKey != nil {
		lastKey = make(map[string]interface{})
		for k, v := range result.LastEvaluatedKey {
			lastKey[k] = formatAttributeValue(v)
		}
	}

	return QueryResult{Items: items, RawItems: rawItems, LastEvaluatedKey: lastKey}, nil
}

func (c *Client) getTableInfo(name string) (TableInfo, error) {
	result, err := c.svc.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: &name,
	})
	if err != nil {
		return TableInfo{}, err
	}

	table := result.Table
	var partitionKey, sortKey string
	schemaFields := make(map[string]bool)

	// Main table key schema
	for _, ks := range table.KeySchema {
		if ks.AttributeName != nil {
			schemaFields[*ks.AttributeName] = true
			switch ks.KeyType {
			case "HASH":
				partitionKey = *ks.AttributeName
			case "RANGE":
				sortKey = *ks.AttributeName
			}
		}
	}

	// GSI key schemas
	for _, gsi := range table.GlobalSecondaryIndexes {
		for _, ks := range gsi.KeySchema {
			if ks.AttributeName != nil {
				schemaFields[*ks.AttributeName] = true
			}
		}
	}

	// Convert map to slice
	var fields []string
	for f := range schemaFields {
		fields = append(fields, f)
	}

	return TableInfo{
		Name:         name,
		Status:       string(table.TableStatus),
		ItemCount:    *table.ItemCount,
		SizeBytes:    *table.TableSizeBytes,
		PartitionKey: partitionKey,
		SortKey:      sortKey,
		SchemaFields: fields,
	}, nil
}
