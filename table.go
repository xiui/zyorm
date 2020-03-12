package zyorm

import "sync"

type TableInfo struct {

	Name string

	RWRuField *sync.RWMutex
	Fields map[string]FieldInfo


}

type FieldInfo struct {

	AttrName string	//属性名字
	FieldName string //字段名
	AsName string //别名
	TableName string //表名
}