package zyorm

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
)

//记录每次sql操作的信息
type Session struct {

	Tx *sql.Tx

	TableName string

	Engine *Engine
	where string
	limit string
	order string
	group string

	joins []string

	args []interface{}

	prepare string	//直接写 sql 时使用

}

func (session *Session) Begin() error {
	var err error
	session.Tx, err = session.Engine.Db.Begin()
	return err
}

func (session *Session) Rollback() error {
	return session.Tx.Rollback()
}

func (session *Session) Commit() error {
	return session.Tx.Commit()
}


func (session *Session) Table(tableName string) *Session {
	session.TableName = tableName
	return session
}

func (session *Session) Prepare(sqlstr string) *Session {

	session.prepare = sqlstr
	return session
}

func (session *Session)Query(args ...interface{}) ([]map[string]string, error) {

	defer session.clearSession()

	if len(session.prepare) < 1 {
		return nil, errors.New("请先调用 Prepare方法")
	}

	session.args = args

	columns, valuess,err := session.getRows(session.prepare)
	if err != nil {
		return nil, err
	}

	var maps []map[string]string

	for _, values := range *valuess {

		mapElement := make(map[string]string)
		for i, column := range columns {
			mapElement[column] = string((*values)[i])
		}

		maps = append(maps, mapElement)
	}

	return maps, nil

}

func (session *Session) Insert(data map[string]interface{}) (int64, error) {

	defer session.clearSession()

	if len(session.TableName) < 1 {
		return 0, errors.New("没有相应的表明")
	}

	if len(data) < 1 {
		return 0, errors.New("参数没有数据")
	}

	var args []interface{}

	kstr := "("
	vstr := "("
	for k, v := range data {

		if len(kstr) > 1 {
			kstr += "," + "`" + k + "`"
			vstr += ",?"
		} else {
			kstr += "`" + k + "`"
			vstr += "?"
		}

		args = append(args, v)

	}


	kstr += ")"
	vstr += ")"

	sqlstr := "INSERT " + session.TableName + kstr + " VALUES " + vstr

	fmt.Println(sqlstr)


	var stmtIns *sql.Stmt
	var err error

	if session.Tx != nil {
		stmtIns, err = session.Tx.Prepare(sqlstr)
	} else {
		stmtIns, err = session.Engine.Db.Prepare(sqlstr)
	}


	if err != nil {
		 return 0, err
	}

	defer stmtIns.Close()

	ret, err := stmtIns.Exec(args...)

	if err != nil {
		return 0, err
	}

	lastInsertId, err := ret.LastInsertId()

	if err != nil {
		return 0, err
	}

	return lastInsertId, nil

}

func (session *Session) Update(data map[string]interface{}) (int64, error) {

	defer session.clearSession()

	if len(session.TableName) < 1 {
		return 0, errors.New("没有相应的表明")
	}

	if len(data) < 1 {
		return 0, errors.New("参数没有数据")
	}

	var args []interface{}

	setStr := ""
	for k, v := range data {

		if len(setStr) > 0 {
			setStr += "," + "`" + k + "`" + "=?"
		} else {
			setStr += "`" + k + "`" + "=?"
		}

		args = append(args, v)

	}


	
	sqlstr := "UPDATE " + session.TableName + " SET " + setStr  //kstr + " VALUES " + vstr

	if len(session.where) > 0 {
		sqlstr += " WHERE " + session.where
		args = append(args, session.args...)
	}

	fmt.Println(args)
	fmt.Println(sqlstr)

	var stmtIns *sql.Stmt
	var err error

	fmt.Println(session.Tx)

	if session.Tx != nil {
		stmtIns, err = session.Tx.Prepare(sqlstr)
	} else {
		stmtIns, err = session.Engine.Db.Prepare(sqlstr)
	}

	fmt.Println(stmtIns)
	fmt.Println(err)

	if err != nil {
		return 0, err
	}

	defer stmtIns.Close()

	ret, err := stmtIns.Exec(args...)

	lastsql,_ :=ret.RowsAffected()

	fmt.Println("ret:", lastsql)
	fmt.Println(err)

	if err != nil {
		return 0, err
	}

	rowsAffected, err := ret.RowsAffected()

	if err != nil {
		return 0, err
	}

	return rowsAffected, nil

}

func (session *Session) Delete() (int64, error) {

	defer session.clearSession()

	if len(session.TableName) < 1 {
		return 0, errors.New("没有相应的表明")
	}

	if len(session.where) < 1 {
		return 0, errors.New("delete 必须设置 where")
	}

	sqlstr := "DELETE FROM " + session.TableName + " WHERE " + session.where

	fmt.Println(sqlstr)

	var stmtIns *sql.Stmt
	var err error

	if session.Tx != nil {
		stmtIns, err = session.Tx.Prepare(sqlstr)
	} else {
		stmtIns, err = session.Engine.Db.Prepare(sqlstr)
	}

	if err != nil {
		return 0, err
	}

	defer stmtIns.Close()

	ret, err := stmtIns.Exec(session.args...)

	if err != nil {
		return 0, err
	}

	rowsAffected, err := ret.RowsAffected()

	if err != nil {
		return 0, err
	}

	return rowsAffected, nil

}

func (session *Session) Find(p interface{}) (bool, error) {

	//因为查 1 条, limit 直接设置成 1
	session.Limit(1)

	defer session.clearSession()


	t, _, realV, err := session.getReflects(p)

	if err != nil {
		return false, err
	}


	sqlstr, err := session.getSqlStr(t)

	if err != nil {
		return false, err
	}

	//根据 sql 查数据
	db := session.Engine.Db

	stmtOut, err := db.Prepare(sqlstr)
	if err != nil {
		log.Printf("prepare error: %s", err)
		return false, err
	}
	defer stmtOut.Close()

	rows, err := stmtOut.Query(session.args...)
	if err != nil {
		log.Printf("Query error: %s", err)
		return false, err
	}

	if err = rows.Err(); err != nil {
		log.Printf("rows Err: %s", err)
		return false, err
	}
	defer rows.Close()


	columns, err := rows.Columns()
	if err != nil {
		log.Printf("get Columns error: %s", err)
		return false, err
	}

	//只查一条, 所以不用循环
	rows.Next()

	//切片是地址, 所以每次都重新创建 values, scanArgs
	values := make([]sql.RawBytes, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	err = rows.Scan(scanArgs...)
	if err != nil {
		log.Printf("get Scan error: %s", err)
		return false, err
	}

	session.setValues(columns, values, t, realV)

	return true, nil

}

func (session *Session) Select(p interface{}) error {

	defer session.clearSession()

	t, v, realV, err := session.getReflects(p)


	if err != nil {
		return err
	}

	sqlstr, err := session.getSqlStr(t)

	if err != nil {
		return err
	}


	//////////////////////////////////////////////////////////////////////////////////////////////////

	db := session.Engine.Db

	stmtOut, err := db.Prepare(sqlstr)
	if err != nil {
		log.Printf("prepare error: %s", err)
		return err
	}

	defer stmtOut.Close()

	rows, err := stmtOut.Query(session.args...)
	if err != nil {
		log.Printf("Query error: %s", err)
		return err
	}

	if err = rows.Err(); err != nil {
		log.Printf("rows Err: %s", err)
		return err
	}

	defer rows.Close()


	columns, err := rows.Columns()
	if err != nil {
		log.Printf("get Columns error: %s", err)
		return err
	}


	elements := make([]reflect.Value, 0)

	//循环输出 mysql 返回数据
	for rows.Next() {

		//切片是地址, 所以每次都重新创建 values, scanArgs
		values := make([]sql.RawBytes, len(columns))

		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			log.Printf("get Scan error: %s", err)
			return err
		}

		session.setValues(columns, values, t, v)
		elements = append(elements, reflect.ValueOf(v.Interface()))

	}
	tmp := reflect.Append(realV, elements...)
	realV.Set(tmp)

	return nil

}

func (session *Session) Count() (int64, error) {

	s := "SELECT COUNT(*) c FROM " + session.TableName

	if len(session.where) > 0 {
		s += " WHERE " + session.where
	}

	m, err := session.Prepare(s).Query(session.args...)

	if err != nil {
		return 0, nil
	}

	if len(m) < 1 {
		return 0, errors.New("获取数量失败")
	}

	count, err := strconv.ParseInt(m[0]["c"], 10, 32)

	return count, err

}

func (session *Session) Where(wheres map[string]interface{}) *Session {

	//如果有内容添加 ()
	if len(wheres) > 0 {

		if len(session.where) > 0 {
			session.where += " and ("
		} else {
			session.where += " ("
		}

		session.manageWhere(wheres)

		session.where += ")"

	}

	return session
}

func (session *Session) OrWhere(wheres map[string]interface{}) *Session {
	//如果有内容添加 ()
	if len(wheres) > 0 {

		if len(session.where) > 0 {
			session.where += " or ("
		} else {
			session.where += " ("
		}

		session.manageWhere(wheres)

		session.where += ")"

	}

	return session
}

func (session *Session) Limit(args ...interface{}) *Session {

	switch len(args) {
	case 1:
		first := args[0]

		if f, ok := first.(string); ok {
			session.limit = f
		} else if f, ok := first.(int); ok  {
			session.limit = strconv.FormatInt(int64(f), 10)
		} else if f, ok := first.(int64); ok  {
			session.limit = strconv.FormatInt(f, 10)
		}

	case 2:
		first := args[0]
		second := args[1]

		var page int64
		var size int64


		if s, ok := second.(string); ok {
			size, _ = strconv.ParseInt(s, 10, 64)
		} else if s, ok := second.(int); ok  {
			size = int64(s)
		} else if s, ok := second.(int64); ok  {
			size = s
		}

		if f, ok := first.(string); ok {
			page, _ = strconv.ParseInt(f, 10, 64)
		} else if f, ok := first.(int); ok  {
			page = int64(f)
		} else if f, ok := first.(int64); ok  {
			page = f
		}

		session.limit = strconv.FormatInt(page * size, 10) + "," + strconv.FormatInt(size, 10)

	}

	return session





}

func (session *Session) Order(order string) *Session {
	session.order = order
	return session
}

func (session *Session) Group(group string) *Session {
	session.group = group
	return session
}

func (session *Session) Join(join string) *Session {
	session.joins = append(session.joins, join)
	return session
}

func (session *Session) manageWhere(wheres map[string]interface{}) {

	isFirst := true

	for k, v := range wheres {

		index := strings.Index(k, ".")

		if index > 0 {

			table := k[:index]
			field := k[index+1:]

			if isFirst {
				isFirst = false
				session.where += " " + table + ".`" + field + "` "
			} else {
				session.where += " and " + table + ".`" + field + "` "
			}

		} else {
			if isFirst {
				isFirst = false
				session.where += " `" + k + "` "
			} else {
				session.where += " and `" + k + "` "
			}
		}





		switch v.(type) {
		case string, int, float64:

			session.where += "=?"

			session.args = append(session.args, v)

		case []interface{}:

			var t string
			if len(v.([]interface{})) > 1 {

				t = v.([]interface{})[0].(string)

				t = strings.ToUpper(t)

				v1 := v.([]interface{})[1]
				switch t {
				case "=", ">", ">=", "<", "<=", "<>", "!=", "LIKE":

					session.where += t + " ? "


					session.args = append(session.args, v1)


				case "IN":

					switch v1.(type) {
					case string, int, float64:
						session.where += t + " (?) "
						session.args = append(session.args, v1)
					case []int:
						session.where += t + " ("

						for i, intv := range v1.([]int) {

							if i == 0 {
								session.where += " ? "
							} else {
								session.where += " ,? "
							}

							session.args = append(session.args, intv)
						}
						session.where += " ) "
					case []string:
						session.where += t + " ( "

						for i, intv := range v1.([]string) {

							if i == 0 {
								session.where += " ? "
							} else {
								session.where += " ,? "
							}

							session.args = append(session.args, intv)
						}
						session.where += " ) "
					case []float64:
						session.where += t + " ( "

						for i, intv := range v1.([]float64) {

							if i == 0 {
								session.where += " ? "
							} else {
								session.where += " ,? "
							}

							session.args = append(session.args, intv)
						}
						session.where += " ) "
					case []interface{}:
						session.where += t + " ( "

						for i, intv := range v1.([]interface{}) {

							if i == 0 {
								session.where += " ? "
							} else {
								session.where += " ,? "
							}

							session.args = append(session.args, intv)
						}
						session.where += " ) "
					}

				case "BETWEEN":

					if len(v.([]interface{})) == 3 {
						v2 := v.([]interface{})[1]
						v3 := v.([]interface{})[2]

						session.where += t + " ? and ? "
						session.args = append(session.args, v2, v3)

					} else if len(v.([]interface{})) < 3 {

						switch v1.(type) {

						case []int:

							if len(v1.([]int)) == 2 {
								session.where += t

								for i, intv := range v1.([]int) {

									if i == 0 {
										session.where += " ? "
									} else {
										session.where += " and ? "
									}

									session.args = append(session.args, intv)
								}
							}
						case []float64:

							if len(v1.([]float64)) == 2 {
								session.where += t

								for i, intv := range v1.([]float64) {

									if i == 0 {
										session.where += " ? "
									} else {
										session.where += " and ? "
									}

									session.args = append(session.args, intv)
								}
							}

						case []string:

							if len(v1.([]string)) == 2 {
								session.where += t

								for i, intv := range v1.([]string) {

									if i == 0 {
										session.where += " ? "
									} else {
										session.where += " and ? "
									}

									session.args = append(session.args, intv)
								}
							}
						case []interface{}:

							if len(v1.([]interface{})) == 2 {
								session.where += t

								for i, intv := range v1.([]interface{}) {

									if i == 0 {
										session.where += " ? "
									} else {
										session.where += " and ? "
									}

									session.args = append(session.args, intv)

								}
							}

						}

					} else {


					}

				}

			} else {

			}

		default:

		}

	}

}

func (session *Session) setValues(columns []string, values []sql.RawBytes, t reflect.Type, v reflect.Value)  {

	tableInfo := session.Engine.Tables[t.Name()]

	for i, column := range columns {

		fieldInfo := tableInfo.Fields[column]

		valueBytes := values[i]

		if valueBytes != nil {
			value := string(valueBytes)

			f := v.FieldByName(fieldInfo.AttrName)

			switch f.Kind() {
			case reflect.String:
				f.SetString(value)
			case reflect.Int:
				intV, e := strconv.ParseInt(value, 10, 64)
				if e != nil {
					f.SetInt(0)
				} else {
					f.SetInt(intV)
				}
			case reflect.Float64:
				floatV, e := strconv.ParseFloat(value,64)
				if e != nil {
					f.SetFloat(0)
				} else {
					f.SetFloat(floatV)
				}
			}
		}

	}

}

//获取
/*
返回参数: 类型, 可修改值的 Value, 最终修改的 Value, err
 */
func (session *Session) getReflects(p interface{}) (reflect.Type, reflect.Value, reflect.Value, error) {

	t := reflect.TypeOf(p)
	realV := reflect.ValueOf(p).Elem()

	if t.Kind() != reflect.Ptr {
		return nil, reflect.Value{}, reflect.Value{},errors.New("参数不是指针类型")
	}


	n := 0
	for t.Kind().String() != "struct" {

		t = t.Elem()
		n ++

		//如果是 &struct{}, n=1; 如果是&[]struct, n=2; 其他的情况不处理
		if n > 2 {
			return nil, reflect.Value{}, reflect.Value{},errors.New("参数不是结构体指针或结构体切片")
		}
	}

	v := reflect.New(t).Elem()

	return t, v, realV, nil

}

func (session *Session) getSqlStr(t reflect.Type) (string, error) {
	tableInfo, ok := session.Engine.Tables[t.Name()]

	if !ok {

		err := session.Engine.registerTable(t)
		if err != nil {
			return "", err
		}

		tableInfo = session.Engine.Tables[t.Name()]
	}


	tableFields := tableInfo.Fields

	var fields []string

	for _, v := range tableFields {



		field := ""

		if v.TableName != "" {
			field += v.TableName + "."
		}



		field += "`"+v.FieldName+"`"



		if v.AsName != "" {
			field += " `" + v.AsName + "`"
		}



		fields = append(fields, field)
	}


	fieldStr := strings.Join(fields, ",")


	sqlstr := "SELECT " + fieldStr + " FROM " + tableInfo.Name

	for _, join := range session.joins {

		sqlstr += " " + join

	}


	if len(session.where) > 0 {
		sqlstr += " WHERE " + session.where
	}

	if len(session.order) > 0 {
		sqlstr += " ORDER BY " + session.order
	}

	if len(session.limit) > 0 {
		sqlstr += " LIMIT " + session.limit
	}

	if len(session.group) > 0 {
		sqlstr += " GROUP BY " + session.group
	}

	fmt.Println(sqlstr)

	return sqlstr, nil
}

func (session *Session) getRows(sqlstr string) ([]string, *[]*[]sql.RawBytes, error) {


	db := session.Engine.Db

	stmtOut, err := db.Prepare(sqlstr)
	if err != nil {
		log.Printf("prepare error: %s", err)
		return nil, nil, err
	}
	defer stmtOut.Close()

	rows, err := stmtOut.Query(session.args...)
	if err != nil {
		log.Printf("Query error: %s", err)
		return nil, nil, err
	}

	if err = rows.Err(); err != nil {
		log.Printf("rows Err: %s", err)
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Printf("get Columns error: %s", err)
		return nil, nil, err
	}


	var allValues = make([]*[]sql.RawBytes, 0)


	//循环输出 mysql 返回数据
	for rows.Next() {

		//切片是地址, 所以每次都重新创建 values, scanArgs
		values := make([]sql.RawBytes, len(columns))

		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]

			fmt.Printf("value-%d: ,%p\n", i, &values[i])
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			log.Printf("get Scan error: %s", err)
			return nil, nil, err
		}

		fmt.Println("cap: ", cap(allValues))
		fmt.Println("len: ", len(allValues))

		fmt.Println("00000000000000000000000000")
		for i, v := range values {
			fmt.Println(columns[i] + "_value: ", v)
			fmt.Println(columns[i] + "_value: ", string(v))

			//fmt.Printf("%s_point: %p, len: %d", columns[i], &v, len(v))

			fmt.Println()
		}
		//TODO 数据量比较大的时候, allValues 中的数据会出错
		allValues = append(allValues, &values)

		//fmt.Printf("values: %p\n", &values)
		fmt.Println("11111111111111111111111111")
		//for _, va := range allValues {
		//
		//	//fmt.Printf("allValues: %p\n", va)
		//	//for i, v := range *va {
		//	//	//fmt.Println(columns[i] + "_value: " + string(v))
		//	//
		//	//	//fmt.Printf("%s_point: %p, len: %d\n", columns[i], &v, len(v))
		//	//}
		//	//fmt.Println()
		//}
		fmt.Println("2222222222222222222222222")

	}



	return columns, &allValues, nil


}

//TODO: 每次增删改查完之后, 清空一下
func (session *Session)clearSession() {

	session.where = ""
	session.args = []interface{}{}
	session.limit = ""
	session.order = ""
	session.group = ""
	session.joins = []string{}

	session.prepare = ""


}
