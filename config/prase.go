package config

import (
	"errors"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

func NewCfg(path string) (*Cfg, error) {
	c := new(Cfg)
	c.SetDefault()

	err := FromFile(string(path), c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Cfg) SetDefault() {
	c.Name = "pm"
	c.Log.Level = "debug"
	c.Web.Mode = "debug"
}

// FromFile 指定配置文件路径path，解析到目标def
func FromFile(path string, dest any) error {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	}

	defer file.Close() //nolint:errcheck // The file is RO
	return FromReader(file, dest)
}

// FromReader loads config from a reader instance.
func FromReader(reader io.Reader, dest any) error {
	_, err := toml.NewDecoder(reader).Decode(dest)
	if err != nil {
		return err
	}

	return nil
}
