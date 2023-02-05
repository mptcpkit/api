package api

import (
	"flag"
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Configuration struct {
	Api struct {
		ScriptsDir string `yaml:"script_dir" env:"MPTCPKIT_SCRIPT_DIR" env-description:"location of scripts for endpoint actions" env-default:"/etc/mptcpkit/endpoints"`
		DryRun     bool   `yaml:"dry" env:"MPTCPKIT_DRYRUN" env-description:"API endpoints don't do effects env-default:false`
		KeyFile    string `yaml:"key_file" env:"MPTCPKIT_KEYFILE" env-description:"API/Services keys"`
	} `yaml:"api"`
	Server struct {
		Host        string `yaml:"host" env:"MPTCPKIT_HOST" env-description:"Address to bind" env-default:"0.0.0.0"`
		Port        string `yaml:"port" env:"MPTCPKIT_PORT" env-description:"Port to bind" env-default:"8080"`
		Https       bool   `yaml:"https" env:"MPTCPKIT_HTTPS" env-description:"Enable HTTPS" env-default:true`
		TLSCertFile string `yaml:"tls_cert" env:"MPTCPKIT_TLS_CERT" env-description:"TLS cert location"`
		TLSCertKey  string `yaml:"tls_key" env:"MPTCPKIT_TLS_KEY" env-description:"TLS key location"`
	} `yaml:"server"`
}

type Args struct {
	ConfigPath string
}

func ProcessArgs(cfg interface{}) Args {
	var a Args
	f := flag.NewFlagSet("Mptcpkit API", 1)
	f.StringVar(&a.ConfigPath, "c", "/etc/mptcpkit/config.yml", "Path to the config file")

	fu := f.Usage
	f.Usage = func() {
		fu()
		envHelp, _ := cleanenv.GetDescription(cfg, nil)
		fmt.Fprintln(f.Output())
		fmt.Fprintln(f.Output(), envHelp)
	}
	f.Parse(os.Args[1:])
	return a
}
