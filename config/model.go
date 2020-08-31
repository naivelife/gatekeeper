package config

import "git.baijiahulian.com/plt/go-common/model/config"

// 验证结构体
type AuthConfig struct {
	AdminName     string `json:"admin_username"`
	AdminPassport string `json:"admin_passport"`
}

type MySQLConfig struct {
	DataSourceName  string `json:"data_source_name"`
	MaxOpenConn     int    `json:"max_open_conn"`
	MaxIdleConn     int    `json:"max_idle_conn"`
	MaxConnLifeTime int    `json:"max_conn_life_time"`
}

type RedisConfig struct {
	Password    string `json:"password"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	DB          int    `json:"db"`
	MaxIdle     int    `json:"max_idle"`
	MaxActive   int    `json:"max_active"`
	IdleTimeout int    `json:"idle_timeout"`
}

type BaseConfig struct {
	DebugModel   string            `json:"debug_model"`
	SysLog       *config.LogConfig `json:"sys_log"`
	AccessLog    *config.LogConfig `json:"access_log"`
	TimeLocation string            `json:"time_location"`
	Interval     int               `json:"interval"`
	Http         *HttpConfig       `json:"http"`
	Cluster      *ClusterConfig    `json:"cluster"`
}

type HttpConfig struct {
	RoutePrefix    string `json:"route_prefix"`
	Addr           string `json:"addr"`
	ReqHost        string `json:"req_host"`
	ReadTimeout    int    `json:"read_timeout"`
	WriteTimeout   int    `json:"write_timeout"`
	MaxHeaderBytes int    `json:"max_header_bytes"`
}

type ClusterConfig struct {
	ClusterIP   string `json:"cluster_ip"`
	ClusterAddr string `json:"cluster_addr"`
	ClusterList string `json:"cluster_list"`
}
