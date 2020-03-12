package main

import (
	"fmt"
	"github.com/xiui/zyorm"
)

var Engine *zyorm.Engine
var err error

func init() {

	if Engine == nil {
		Engine, err = zyorm.NewEngine(zyorm.DnsConf{
			Username: "root",
			Password: "root",
			Ip: "1.2.3.4",
			Port: "3306",
			TableName: "test",
			ParamsStr: "charset=utf8",
		})

		if err != nil {
			panic("数据库连接失败")
		}
	}

}

func main()  {

	count, err := Engine.Table("bill").Where(map[string]interface{}{
		"user_id": 11,
	}).Count()
	fmt.Println(err)
	fmt.Println(count)

	fmt.Println(Engine)
}