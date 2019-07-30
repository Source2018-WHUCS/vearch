// Copyright 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package config

import (
	stdJson "encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/tiglabs/baudengine/util/bytes"
	"github.com/tiglabs/baudengine/util/cbjson"
)

// Config configuration information reading tool class
type Config struct {
	data map[string]interface{}
	Raw  []byte
}

func newConfig() *Config {
	result := new(Config)
	result.data = make(map[string]interface{})
	return result
}

// LoadConfigFile loads config information from a JSON file
func LoadConfigFile(filename string) *Config {
	result := newConfig()
	err := result.parse(filename)
	if err != nil {
		log.Fatalf("error loading config file %s: %s", filename, err)
	}
	return result
}

// LoadConfigString loads config information from a JSON string
func LoadConfigString(s string) *Config {
	result := newConfig()
	err := cbjson.Unmarshal([]byte(s), &result.data)
	if err != nil {
		log.Fatalf("error parsing config string %s: %s", s, err)
	}
	return result
}

func (c *Config) parse(fileName string) error {
	jsonFileBytes, err := ioutil.ReadFile(fileName)
	c.Raw = jsonFileBytes
	if err == nil {
		err = cbjson.Unmarshal(jsonFileBytes, &c.data)
	}
	return err
}

// GetString Returns a string for the config variable key
func (c *Config) GetString(key string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}

	result, present := c.data[key]
	if !present {
		return ""
	}
	return result.(string)
}

func (c *Config) GetRaw(key string) map[string]interface{} {
	result, present := c.data[key]
	if !present {
		return nil
	}
	return result.(map[string]interface{})
}

// GetFloat Returns a float for the config variable key
func (c *Config) GetFloat(key string) float64 {
	if env := os.Getenv(key); env != "" {
		if val, err := strconv.ParseFloat(env, 64); err == nil {
			return val
		}
	}

	x, ok := c.data[key]
	if !ok {
		return -1
	}
	return x.(float64)
}

func (c *Config) GetNumber(key string) stdJson.Number {
	if env := os.Getenv(key); env != "" {
		return stdJson.Number(env)
	}

	x, ok := c.data[key]
	if !ok {
		return stdJson.Number("")
	}
	return x.(stdJson.Number)
}

// GetBool Returns a bool for the config variable key
func (c *Config) GetBool(key string) bool {
	if env := os.Getenv(key); env != "" {
		if strings.EqualFold(env, "true") {
			return true
		}
		return false
	}

	x, ok := c.data[key]
	if !ok {
		return false
	}
	return x.(bool)
}

// GetArray Returns an array for the config variable key
func (c *Config) GetArray(key string) []interface{} {
	if env := os.Getenv(key); env != "" {
		var data interface{}
		if err := cbjson.Unmarshal(bytes.StringToByte(env), &data); err == nil {
			return data.([]interface{})
		}
	}

	result, present := c.data[key]
	if !present {
		return []interface{}(nil)
	}
	return result.([]interface{})
}
