package appcfg

import (
	"encoding/json"
	"sync"

	"github.com/kaoriel-studio/netwick/kb"
)

type Hog struct {
	Process string `json:"process"`
	Name    string `json:"name"`
}

type Driver struct {
	File       string `json:"file"`
	Vendor     string `json:"vendor"`
	MinVersion string `json:"minVersion"`
}

type Game struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Yaw  float64 `json:"yaw"`
}

type Config struct {
	VPNPatterns         []string `json:"vpnPatterns"`
	NetworkHogs         []Hog    `json:"networkHogs"`
	IncompatibleDrivers []Driver `json:"incompatibleDrivers"`
	VMIndicators        []string `json:"vmIndicators"`
	FreeSpaceWarnGB     uint64   `json:"freeSpaceWarnGB"`
	Sensitivity         struct {
		Note  string `json:"note"`
		Games []Game `json:"games"`
	} `json:"sensitivity"`
}

var (
	once sync.Once
	cfg  Config
)

func Get() Config {
	once.Do(func() {
		if err := json.Unmarshal(kb.ConfigJSON, &cfg); err != nil {
			panic(err)
		}
	})
	return cfg
}
