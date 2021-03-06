package zyorm

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type Engine struct {
	db *sql.DB

	ShowSql bool

	//用 select 查询时, 用 var a type时, 如果没有数据, 返回后 json 化的时候, 会解析成 null, 如果想解析成空数组 [], 这里加个判断, 在查不到数据时, 处理成空数组
	SelectNilSlice2EmptySlice bool

	rwMuTables *sync.RWMutex
	tables map[string]TableInfo

}

type DnsConf struct {
	Username string
	Password string
	Ip string
	Port string
	TableName string
	ParamsStr string
}

func NewEngine(dnsConf DnsConf) (*Engine, error) {

	dsn := dnsConf.Username + ":" + dnsConf.Password + "@tcp(" + dnsConf.Ip + ":" + dnsConf.Port + ")/" + dnsConf.TableName + "?" + dnsConf.ParamsStr

	db, err := sql.Open("mysql", dsn)

	if err != nil {
		return nil, err
	}


	//设置数据库空闲连接
	db.SetMaxIdleConns(20)
	//设置最大打开数量
	db.SetMaxOpenConns(20)

	//直接判断是不是能连接成功
	err = db.Ping()
	if err != nil {
		log.Printf("ping error: %s", err)

		return nil, err
	}

	engine := &Engine{db: db, rwMuTables:new(sync.RWMutex), tables: make(map[string]TableInfo)}

	return engine, nil

}

func (engine *Engine) NewSession() (*Session) {
	session := engine.createSession()

	return session
}

func (engine *Engine) Table(tableName string) *Session {
	session := engine.createSession()
	return session.Table(tableName)
}

func (engine *Engine) Prepare(sqlstr string) *Session {
	session := engine.createSession()
	return session.Prepare(sqlstr)
}


func (engine *Engine) createSession() *Session {
	return &Session{Engine: engine}
}

func (engine *Engine) Find(p interface{}) (bool, error) {

	session := engine.createSession()

	return session.Find(p)
}

func (engine *Engine) Select(p interface{}) error {
	session := engine.createSession()
	return session.Select(p)
}

func (engine *Engine) Fields(fields string) *Session {

	session := engine.createSession()
	return session.Fields(fields)

}

func (engine *Engine) Where(wheres map[string]interface{}) *Session {

	session := engine.createSession()

	return session.Where(wheres)

}

func (engine *Engine) OrWhere(wheres map[string]interface{}) *Session {

	session := engine.createSession()

	return session.OrWhere(wheres)

}

func (engine *Engine) Limit(args ...interface{}) *Session {

	session := engine.createSession()

	return session.Limit(args...)
}

func (engine *Engine) Order(order string) *Session {

	session := engine.createSession()

	return session.Order(order)
}

func (engine *Engine) Group(group string) *Session {

	session := engine.createSession()

	return session.Group(group)
}

func (engine *Engine) Join(join string, args... interface{}) *Session {

	session := engine.createSession()

	return session.Join(join, args...)
}

func (engine *Engine) registerTable(t reflect.Type) error {

	engine.rwMuTables.Lock()
	defer engine.rwMuTables.Unlock()

	structName := t.Name()

	var tableName = structName

	tableInfo := TableInfo{
		RWRuField: new(sync.RWMutex),
		Fields: make(map[string]FieldInfo),
	}

	ts := []reflect.Type{t}

	hasIsTable := false
	for len(ts) > 0 {
		t := ts[0]
		ts = ts[1:]

		for i := 0; i < t.NumField(); i ++ {

			if t.Field(i).Type.Kind().String() == "struct" {
				ts = append(ts, t.Field(i).Type)
				continue
			}

			attributeName := t.Field(i).Name
			fieldName := t.Field(i).Tag.Get("zyfield")
			asName := t.Field(i).Tag.Get("zyas")

			//获取 zytable tag 中的 表名, 如果没有, 就使用 tableName
			zytableName := t.Field(i).Tag.Get("zytable")

			zyisTableName := t.Field(i).Tag.Get("zyis_tablename")
			if len(zyisTableName) > 0 {
				isTablename, err := strconv.ParseBool(zyisTableName)
				if err != nil {
					log.Println(err)
				}

				//如果指明此字段表示表名, 则不添加了
				if isTablename {

					if hasIsTable {
						log.Println("zyis_tablename more than 1, please check you code")
						continue
					}

					hasIsTable = true

					tableName = strings.ToLower(attributeName)
					continue
				}
			}



			if len(zytableName) < 1 {
				zytableName = strings.ToLower(tableName)
			}


			if fieldName == "-" {
				continue
			}

			if fieldName == "" {
				fieldName = strings.ToLower(attributeName)
			}

			//
			if asName == "" {
				asName = fieldName
			}


			tableInfo.RWRuField.Lock()

			tableInfo.Fields[asName] = FieldInfo{
				AttrName: attributeName,
				FieldName: fieldName,
				AsName: asName,
				TableName: zytableName,
			}

			tableInfo.RWRuField.Unlock()
		}
	}


	tableInfo.Name = strings.ToLower(tableName)
	engine.tables[structName] = tableInfo

	return nil
}



