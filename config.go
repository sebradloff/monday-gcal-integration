package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Port         int    `json:"port"`
	Env          string `json:"env"`
	ClientID     string `json:"client_id"`
	Secret       string `json:"secret"`
	BoardID      int    `json:"board_id"`
	MondayAPIKey string `json:"monday_api_key"`
}

func LoadConfig(useFile bool) Config {
	if useFile {
		configFile := ".config"
		f, err := os.Open(configFile)
		if err != nil {
			fmt.Printf("file %s with the config is required", configFile)
			panic(err)
		}
		var c Config
		dec := json.NewDecoder(f)
		err = dec.Decode(&c)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Successfully loaded %s", configFile)
		return c
	}
	panic("load from env at some point")
}
