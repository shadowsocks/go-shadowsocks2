package config

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"
)

type JsonConfig struct {
    Type      string
    Server    string
    Cipher    string
    Key       string
    Password  string
    Redir     string
    Redir6    string
    Socks     string
    UDPSocks  bool
    TCPtun    string
    UDP       bool
    UDPtun    string
    RedirTCP  string
    RedirTCP6 string
    Verbose   bool
}

func ParseConfig(file string) (JsonConfig, error) {
    f, err := os.Open(file)

    if err != nil {
        return JsonConfig{}, err
    }

    decoder := json.NewDecoder(f)

    config := JsonConfig{}

    err = decoder.Decode(&config)

    if err != nil {
        return JsonConfig{}, err
    }

    return config, nil
}

func (conf JsonConfig) IsClient() bool {
    switch strings.ToLower(conf.Type) {
    case "client", "c":
        return true

    case "server", "s":
        return false

    default:
        panic(fmt.Sprintf("illegal config type: %s", conf.Type))
    }
}
