package config

import (
    "os"
    "encoding/json"
)

type JsonConfig struct {
    Client    string
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
