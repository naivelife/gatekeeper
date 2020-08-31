module gatekeeper

go 1.11

require (
	git.apache.org/thrift.git v0.12.0 // indirect
	git.baijiahulian.com/plt/go-common v0.0.0-20200706023435-cebeb8c04bbf
	github.com/BurntSushi/toml v0.3.1
	github.com/didi/gatekeeper v0.1.1
	github.com/e421083458/golang_common v1.0.7
	github.com/e421083458/gorm v1.0.1
	github.com/garyburd/redigo v1.6.0
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.3.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/jinzhu/gorm v1.9.15
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/mattn/go-isatty v0.0.8 // indirect
	github.com/pkg/errors v0.8.1
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/viper v1.4.0
	github.com/tidwall/gjson v1.2.1
	github.com/tidwall/match v1.0.1 // indirect
	github.com/tidwall/pretty v0.0.0-20190325153808-1166b9ac2b65 // indirect
	github.com/tidwall/sjson v1.0.4
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v8 v8.18.2
)

//Compatible go 1.11
replace github.com/gin-contrib/sse v0.1.0 => github.com/e421083458/sse v0.1.1

replace golang.org/x/sys v0.0.0-20190412213103-97732733099d => github.com/e421083458/sys v0.0.1
