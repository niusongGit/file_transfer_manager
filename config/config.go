package config

import (
	"bytes"
	"encoding/json"
	"file_transfer_manager/pkg/utils"
	"os"
	"path/filepath"
)

const (
	Path_config = "conf/config.json"
)

var (
	TcpIP   = "0.0.0.0"
	TcpPort = 8088
	WebAddr = "0.0.0.0"
	WebPort = 8080
	LogPath = "logs/log.txt"
)

type Config struct {
	TcpIP   string `json:"TcpIP"`   //ip地址
	TcpPort int    `json:"TcpPort"` //监听端口
	WebAddr string `json:"WebAddr"` //
	WebPort int    `json:"WebPort"` //
	LogPath string `json:"LogPath"`
}

func Step() {
	ok, err := utils.PathExists(Path_config)
	if err != nil {
		panic("检查配置文件错误：" + err.Error())
		return
	}

	if !ok {
		panic("检查配置文件错误")
		return
	}

	bs, err := os.ReadFile(filepath.Join(Path_config))
	if err != nil {
		panic("读取配置文件错误：" + err.Error())
		return
	}

	cfi := new(Config)

	decoder := json.NewDecoder(bytes.NewBuffer(bs))
	decoder.UseNumber()
	err = decoder.Decode(cfi)
	if err != nil {
		panic("解析配置文件错误：" + err.Error())
		return
	}

	if len(cfi.TcpIP) > 0 {
		TcpIP = cfi.TcpIP
	}
	if cfi.TcpPort > 0 {
		TcpPort = cfi.TcpPort
	}
	if len(cfi.WebAddr) > 0 {
		WebAddr = cfi.WebAddr
	}
	if cfi.WebPort > 0 {
		WebPort = cfi.WebPort
	}

}
