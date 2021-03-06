package util

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/harbinzhang/goRainbow/core/protocol"
)

// ContextProvider is for loading rainbow config from config.json.
type ContextProvider struct {
	filename string
}

// Init is for
func (cp *ContextProvider) Init() {
	cp.filename = os.Getenv("configPath")
}

// GetConf is for
func (cp *ContextProvider) GetConf() protocol.Config {
	// Prepare config file
	var conf protocol.Config
	configFile, _ := os.Open(cp.filename)
	defer configFile.Close()
	decoder := json.NewDecoder(configFile)
	if err := decoder.Decode(&conf); err != nil {
		panic("Err decode config: " + err.Error())
	}
	return conf
}

// GetPostfix is for
func (cp *ContextProvider) GetPostfix() string {

	conf := cp.GetConf()

	// Prepare tags
	department := "department=" + conf.Service.Department
	serviceName := "service_name=" + conf.Service.Name
	metricFormat := "metric_format=" + conf.Translator.MetricFormat

	// Prepare tags from env variables
	var dataCenter string
	if os.Getenv("DATACENTER") != "" {
		dataCenter = "data_center=" + os.Getenv("DATACENTER")
	} else {
		dataCenter = "data_center=slv"
	}
	var planet string
	if os.Getenv("ENV") != "" {
		planet = "planet=" + os.Getenv("ENV")
	} else {
		planet = "planet=test"
	}

	dcaZone := "dca_zone=local"
	source := "source=fjord-burrow"

	// postfix := "source=192.168.3.169 data_center=slv dca_zone=local department=fjord planet=sbx888 service_name=porter_rainbow porter_tools=porter-rainbow"
	postfix := strings.Join([]string{source, dataCenter, dcaZone, department, planet, serviceName, metricFormat}, " ")

	return postfix
}

// GetBlacklist is for
func (cp *ContextProvider) GetBlacklist() string {
	conf := cp.GetConf()
	return conf.Consumer.Blacklist
}
