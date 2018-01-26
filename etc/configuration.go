package etc

import "github.com/cloudtask/cloudtask-agent/cache"
import "github.com/cloudtask/libtools/gounits/logger"
import "github.com/cloudtask/libtools/gounits/system"
import "github.com/cloudtask/libtools/gzkwrapper"
import "github.com/cloudtask/common/models"
import "gopkg.in/yaml.v2"

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	SystemConfig *Configuration       = nil
	ServerConfig *models.ServerConfig = nil
)

var (
	ErrConfigFileNotFound      = errors.New("config file not found.")
	ErrConfigGenerateFailure   = errors.New("config file generated failure.")
	ErrConfigFormatInvalid     = errors.New("config file format invalid.")
	ErrConfigServerDataInvalid = errors.New("config server data invalid.")
)

// Configuration is exported
type Configuration struct {
	sync.RWMutex
	Version      string `yaml:"version" json:"version"`
	PidFile      string `yaml:"pidfile" json:"pidfile"`
	RetryStartup bool   `yaml:"retrystartup" json:"retrystartup"`

	Cluster struct {
		DataCenter string `yaml:"datacenter" json:"datacenter"`
		Hosts      string `yaml:"hosts" json:"hosts"`
		Root       string `yaml:"root" json:"root"`
		Device     string `yaml:"device" json:"device"`
		Runtime    string `yaml:"runtime" json:"runtime"`
		OS         string `yaml:"os" json:"os"`
		Platform   string `yaml:"platform" json:"platform"`
		Pulse      string `yaml:"pulse" json:"pulse"`
		Threshold  int    `yaml:"threshold" json:"threshold"`
	} `yaml:"cluster" json:"cluster"`

	API struct {
		Hosts      []string `yaml:"hosts" json:"hosts"`
		EnableCors bool     `yaml:"enablecors" json:"enablecors"`
	} `yaml:"api" json:"api"`

	Cache struct {
		MaxJobs       int    `yaml:"maxjobs" json:"maxjobs"`
		SaveDirectory string `yaml:"savedirectory" json:"savedirectory"`
		AutoClean     bool   `yaml:"autoclean" json:"autoclean"`
		CleanInterval string `yaml:"cleaninterval" json:"cleaninterval"`
		PullRecovery  string `yaml:"pullrecovery" json:"pullrecovery"`
	} `yaml:"cache" json:"cache"`

	Logger struct {
		LogFile  string `yaml:"logfile" json:"logfile"`
		LogLevel string `yaml:"loglevel" json:"loglevel"`
		LogSize  int64  `yaml:"logsize" json:"logsize"`
	} `yaml:"logger" json:"logger"`
}

// New is exported
func New(file string) error {

	if file != "" {
		if !system.FileExist(file) {
			cloudtaskENV, _ := os.LookupEnv("CLOUDTASK")
			if cloudtaskENV == "" {
				return ErrConfigFileNotFound
			}
			fileName := filepath.Base(file)
			if _, err := system.FileCopy("./etc/"+cloudtaskENV+"/"+fileName, file); err != nil {
				return ErrConfigGenerateFailure
			}
			log.Printf("[#etc#] ENV CLOUDTASK: %s\n", cloudtaskENV)
		}
	}

	buf, err := readConfigurationFile(file)
	if err != nil {
		return fmt.Errorf("config read %s", err.Error())
	}

	conf := &Configuration{RetryStartup: true}
	if err := yaml.Unmarshal(buf, conf); err != nil {
		return ErrConfigFormatInvalid
	}

	if err = conf.parseEnv(); err != nil {
		return fmt.Errorf("config parse env %s", err.Error())
	}

	parseDefaultParmeters(conf)
	SystemConfig = conf
	log.Printf("[#etc#] version: %s\n", SystemConfig.Version)
	log.Printf("[#etc#] pidfile: %s\n", SystemConfig.PidFile)
	log.Printf("[#etc#] retrystartup: %s\n", strconv.FormatBool(SystemConfig.RetryStartup))
	log.Printf("[#etc#] cluster: %+v\n", SystemConfig.Cluster)
	log.Printf("[#etc#] APIlisten: %+v\n", SystemConfig.API)
	log.Printf("[#etc#] cache: %+v\n", SystemConfig.Cache)
	log.Printf("[#etc#] logger: %+v\n", SystemConfig.Logger)
	return nil
}

//SaveServerConfig is exported
func SaveServerConfig(data []byte) error {

	if SystemConfig != nil {
		serverConfigs := models.ServerConfigs{}
		if err := models.ServerConfigsDeCode(data, &serverConfigs); err != nil {
			return err
		}
		for key, value := range serverConfigs {
			serverConfigs[strings.ToUpper(key)] = value
		}
		configKey := strings.ToUpper(SystemConfig.Cluster.DataCenter)
		serverConfig := serverConfigs[configKey]
		if serverConfig == nil {
			return ErrConfigServerDataInvalid
		}
		SystemConfig.Lock()
		ServerConfig = serverConfig
		SystemConfig.Unlock()
	}
	return nil
}

//PidFile is exported
func PidFile() string {

	if SystemConfig != nil {
		return SystemConfig.PidFile
	}
	return ""
}

//RetryStartup is exported
func RetryStartup() bool {

	if SystemConfig != nil {
		return SystemConfig.RetryStartup
	}
	return false
}

//ClusterArgs is exported
func ClusterArgs() *gzkwrapper.WorkerArgs {

	if SystemConfig != nil {
		return &gzkwrapper.WorkerArgs{
			Hosts:      SystemConfig.Cluster.Hosts,
			Root:       SystemConfig.Cluster.Root,
			Device:     SystemConfig.Cluster.Device,
			DataCenter: SystemConfig.Cluster.DataCenter,
			Location:   SystemConfig.Cluster.Runtime,
			OS:         SystemConfig.Cluster.OS,
			Platform:   SystemConfig.Cluster.Platform,
			APIAddr:    SystemConfig.API.Hosts[0],
			Pulse:      SystemConfig.Cluster.Pulse,
			Threshold:  SystemConfig.Cluster.Threshold,
		}
	}
	return nil
}

//CacheArgs is exported
func CacheArgs() *cache.CacheArgs {

	if SystemConfig != nil {
		return &cache.CacheArgs{
			MaxJobs:       SystemConfig.Cache.MaxJobs,
			SaveDirectory: SystemConfig.Cache.SaveDirectory,
			AutoClean:     SystemConfig.Cache.AutoClean,
			CleanInterval: SystemConfig.Cache.CleanInterval,
			PullRecovery:  SystemConfig.Cache.PullRecovery,
		}
	}
	return nil
}

//LoggerArgs is exported
func LoggerArgs() *logger.Args {

	if SystemConfig != nil {
		return &logger.Args{
			FileName: SystemConfig.Logger.LogFile,
			Level:    SystemConfig.Logger.LogLevel,
			MaxSize:  SystemConfig.Logger.LogSize,
		}
	}
	return nil
}

func readConfigurationFile(file string) ([]byte, error) {

	fd, err := os.OpenFile(file, os.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}

	defer fd.Close()
	buf, err := ioutil.ReadAll(fd)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func parseDefaultParmeters(conf *Configuration) {

	if conf.Cluster.DataCenter == "" {
		conf.Cluster.DataCenter = models.CLUSTER_CROSS_DC
	}

	if conf.Cluster.Pulse == "" {
		conf.Cluster.Pulse = "8s"
	}

	if conf.Cluster.Threshold == 0 {
		conf.Cluster.Threshold = 1
	}

	if len(conf.API.Hosts) == 0 {
		conf.API.Hosts = []string{":8600"}
	}

	if conf.Cache.MaxJobs == 0 {
		conf.Cache.MaxJobs = 255
	}

	if conf.Cache.CleanInterval == "" {
		conf.Cache.CleanInterval = "30m"
	}

	if conf.Cache.PullRecovery == "" {
		conf.Cache.PullRecovery = "300s"
	}

	if conf.Logger.LogLevel == "" {
		conf.Logger.LogLevel = "info"
	}

	if conf.Logger.LogSize == 0 {
		conf.Logger.LogSize = 20971520
	}
}