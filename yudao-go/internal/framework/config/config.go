// Package config 负责加载与持有应用配置。移植标准：配置启动期一次性加载并注入，禁止运行时全局读取。
package config

import "github.com/spf13/viper"

type Config struct {
	App       App       `mapstructure:"app"`
	Server    Server    `mapstructure:"server"`
	Database  Database  `mapstructure:"database"`
	Redis     Redis     `mapstructure:"redis"`
	EventBus  EventBus  `mapstructure:"eventbus"`
	WebSocket WebSocket `mapstructure:"websocket"`
}

type App struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

func (a App) IsProd() bool { return a.Env == "prod" }

type Server struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type Database struct {
	DSN                string `mapstructure:"dsn"`
	MaxOpenConns       int    `mapstructure:"maxOpenConns"`
	MaxIdleConns       int    `mapstructure:"maxIdleConns"`
	ConnMaxLifetimeSec int    `mapstructure:"connMaxLifetimeSec"`
	SlowThresholdMs    int    `mapstructure:"slowThresholdMs"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type EventBus struct {
	Type       string `mapstructure:"type"`
	Workers    int    `mapstructure:"workers"`
	BufferSize int    `mapstructure:"bufferSize"`
}

type WebSocket struct {
	Enable bool   `mapstructure:"enable"`
	Path   string `mapstructure:"path"`
}

// Load 从指定路径读取 yaml 配置并支持环境变量覆盖。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
