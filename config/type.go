package config

type Cfg struct {
	Name    string     `toml:"server"`
	Log     LogCfg     `toml:"log"`
	Ws      WsCfg      `toml:"ws"`
	Http    HttpCfg    `toml:"http"`
	Web     WebCfg     `toml:"web"`
	Binance BinanceCfg `toml:"binance"`
}

type LogCfg struct {
	Level string `toml:"level"`
}

type WebCfg struct {
	Mode string `toml:"mode"` // release/debug/test
	Addr string `toml:"addr"` // 192.168.1.100:8080
}

type WsCfg struct {
	Proxy     string `toml:"proxy"`
	WriteWait int    `toml:"write_wait"`
}

type HttpCfg struct {
	Proxy string `toml:"proxy"`
}

type BinanceCfg struct {
	ApiKey            string `toml:"api_key"`
	SecretKey         string `toml:"secret_key"`
	Ed25519ApiKey     string `toml:"ed25519_api_key"`
	Ed25519PrivateKey string `toml:"ed25519_private_key"`
}
