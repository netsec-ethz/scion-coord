// Copyright 2016 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func (as *SCIONLabAS) TableName() string {
	return "scion_lab_as"
}

func (sb *SCIONBox) TableName() string {
	return "scion_box"
}

func (il *ISDLocation) TableName() string {
	return "isd_location"
}

func init() {
	orm.RegisterDriver("mysql", orm.DRMySQL)
	orm.RegisterDataBase("default", "mysql",
		fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8&parseTime=true",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName),
		config.DBMaxConnections, config.DBMaxIdle)

	// prints the queries
	orm.Debug = false

	// register the models
	orm.RegisterModel(new(user), new(Account), new(JoinRequest), new(ConnRequest),
		new(JoinReply), new(ConnReply), new(SCIONLabAS), new(AttachmentPoint), new(Connection),
		new(SCIONBox), new(ISDLocation))

	// print verbose logs when generating the tables
	verbose := true

	// DANGER: force table re-creation
	force := false

	err := orm.RunSyncdb("default", force, verbose)
	if err != nil {
		fmt.Println(err)
	}

	// instantiate a new ORM object for executing the queries
	o = orm.NewOrm()
	o.Using("default")
}
