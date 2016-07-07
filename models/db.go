package models

import (
	"fmt"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
	"github.com/netsec-ethz/scion-coord/config"
)

var (
	o orm.Ormer
)

func init() {
	orm.RegisterDriver("mysql", orm.DRMySQL)
	orm.RegisterDataBase("default", "mysql",
		fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8&parseTime=true",
			config.DB_USER, config.DB_PASS, config.DB_HOST, config.DB_PORT, config.DB_NAME), config.DB_MAX_CONNECTIONS, config.DB_MAX_IDLE)

	// prints the queries
	orm.Debug = false

	// register the models
	orm.RegisterModel(new(user), new(Account), new(As),
		new(JoinRequest), new(JoinRequestMapping), new(ConnRequest),
		new(ConnRequestMapping), new(JoinReply), new(ConnReply))

	// priont verbose logs when generating the tables
	verbose := true

	// DANGER: force table re-creation
	force := false

	err := orm.RunSyncdb("default", force, verbose)
	if err != nil {
		fmt.Println(err)
	}

	// instanciate a new ORM object for executing the queries
	o = orm.NewOrm()
	o.Using("default")
}
