package tui

import "ddb-explorer/aws"

type tableLoadSuccessMsg struct {
	tables []aws.TableInfo
}

type tableLoadErrorMsg struct {
	err error
}

type querySuccessMsg struct {
	tableName string
	result    aws.QueryResult
}

type queryErrorMsg struct {
	tableName string
	err       error
}

type scanSuccessMsg struct {
	tableName string
	result    aws.QueryResult
}

type scanErrorMsg struct {
	tableName string
	err       error
}
