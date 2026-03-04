package tui

import "ddb-explorer/aws"

type tableLoadSuccessMsg struct {
	requestID uint64
	tables    []aws.TableInfo
}

type tableLoadErrorMsg struct {
	requestID uint64
	err       error
}

type querySuccessMsg struct {
	requestID uint64
	tableName string
	result    aws.QueryResult
}

type queryErrorMsg struct {
	requestID uint64
	tableName string
	err       error
}

type scanSuccessMsg struct {
	requestID uint64
	tableName string
	result    aws.QueryResult
}

type scanErrorMsg struct {
	requestID uint64
	tableName string
	err       error
}
