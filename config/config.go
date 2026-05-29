package config

import (
	"strings"

	"github.com/spf13/viper"
)

type App struct {
	AppPort    string `json:"app_port"`
	AppEnv     string `json:"app_env"`
	AppName    string `json:"app_name"`
	WebPrefork bool   `json:"web_prefork"`
	LogLevel   string `json:"log_level"`

	JwtSecretKey string `json:"jwt_secret_key"`

	GatewaySecretKey  string `json:"gateway_secret_key"`
	RequestApiGAteway string `json:"request_api_gateway"`

	ServerTimeOut     int    `json:"server_timeout"`
	ProductServiceUrl string `json:"product_service_url"`
	UserServiceUrl    string `json:"user_service_url"`

	LatitudeRef  string `json:"latitude_ref"`
	LongitudeRef string `json:"longitude_ref"`
	MaxDistance  int    `json:"max_distance"`
}

type PsqlDB struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	DBName    string `json:"db_name"`
	DBMaxOpen int    `json:"db_max_open"`
	DBMaxIdle int    `json:"db_max_idle"`
}

type Kafka struct {
	AutoOffsetReset  string   `json:"kafka_auto_offset_reset"`
	BootstrapServers []string `json:"kafka_bootstrap_servers"`
	GroupID          string   `json:"kafka_group_id"`
	ProducerEnabled  bool     `json:"kafka_producer_enabled"`
}

type Topic struct {
	ProductUpdateStock      string `json:"product_update_stock"`
	OrderPublish            string `json:"order_publish"`
	EmailUpdateStatus       string `json:"email_update_status"`
	PublisherDeleteOrder    string `json:"publisher_delete_order"`
	PublisherPaymentSuccess string `json:"publisher_payment_success"`
	PublisherUpdateStatus   string `json:"publisher_update_status"`
}

type Redis struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Password string `json:"password"`
}

type OpenSearch struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	App        App        `json:"app"`
	Psql       PsqlDB     `json:"psql"`
	Kafka      Kafka      `json:"kafka"`
	Topic      Topic      `json:"topic"`
	Redis      Redis      `json:"redis"`
	OpenSearch OpenSearch `json:"opensearch"`
}

func NewConfig() *Config {
	return &Config{
		App: App{
			AppPort:           viper.GetString("APP_PORT"),
			AppEnv:            viper.GetString("APP_ENV"),
			AppName:           viper.GetString("APP_NAME"),
			WebPrefork:        viper.GetBool("WEB_PREFORK"),
			LogLevel:          viper.GetString("LOG_LEVEL"),
			JwtSecretKey:      viper.GetString("JWT_SECRET_KEY"),
			GatewaySecretKey:  viper.GetString("GATEWAY_SECRET_KEY"),
			RequestApiGAteway: viper.GetString("REQUEST_API_GATEWAY"),
			ServerTimeOut:     viper.GetInt("SERVER_TIMEOUT"),
			ProductServiceUrl: viper.GetString("PRODUCT_SERVICE_URL"),
			UserServiceUrl:    viper.GetString("USER_SERVICE_URL"),
			LatitudeRef:       viper.GetString("LATITUDE_REF"),
			LongitudeRef:      viper.GetString("LONGITUDE_REF"),
			MaxDistance:       viper.GetInt("MAX_DISTANCE"),
		},
		Psql: PsqlDB{
			Host:      viper.GetString("DATABASE_HOST"),
			Port:      viper.GetString("DATABASE_PORT"),
			User:      viper.GetString("DATABASE_USER"),
			Password:  viper.GetString("DATABASE_PASSWORD"),
			DBName:    viper.GetString("DATABASE_NAME"),
			DBMaxOpen: viper.GetInt("DATABASE_MAX_OPEN_CONNECTION"),
			DBMaxIdle: viper.GetInt("DATABASE_MAX_IDLE_CONNECTION"),
		},
		Kafka: Kafka{
			AutoOffsetReset:  viper.GetString("KAFKA_AUTO_OFFSET_RESET"),
			BootstrapServers: strings.Split(viper.GetString("KAFKA_BOOTSTRAP_SERVERS"), ","),
			GroupID:          viper.GetString("KAFKA_GROUP_ID"),
			ProducerEnabled:  viper.GetBool("KAFKA_PRODUCER_ENABLED"),
		},
		Topic: Topic{
			ProductUpdateStock:      viper.GetString("product-update-stock"),
			OrderPublish:            viper.GetString("order-publish"),
			EmailUpdateStatus:       viper.GetString("email-update-status-order"),
			PublisherDeleteOrder:    viper.GetString("delete-order"),
			PublisherPaymentSuccess: viper.GetString("payment-success"),
			PublisherUpdateStatus:   viper.GetString("update-status-order"),
		},
		Redis: Redis{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
		},
		OpenSearch: OpenSearch{
			Host:     viper.GetString("OPENSEARCH_HOST"),
			Username: viper.GetString("OPENSEARCH_USERNAME"),
			Password: viper.GetString("OPENSEARCH_PASSWORD"),
		},
	}
}
