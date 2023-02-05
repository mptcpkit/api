package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/heartwilltell/log"
	"gopkg.in/yaml.v2"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/tompinn23/mptcpkit/api"
	"github.com/unrolled/secure"

	"github.com/gin-gonic/gin"
	"github.com/pjebs/restgate"
)

func main() {
	var cfg api.Configuration

	args := api.ProcessArgs(&cfg)

	if err := cleanenv.ReadConfig(args.ConfigPath, &cfg); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if os.Getenv("MPTCPKIT_MODE") != "DEV" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := log.New()
	r := gin.Default()
	sec := secure.New()
	secFunc := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			err := sec.Process(c.Writer, c.Request)
			if err != nil {
				c.Abort()
				return
			}

			if status := c.Writer.Status(); status > 300 && status < 399 {
				c.Abort()
			}
		}
	}()
	r.Use(secFunc)
	yamlF, err := os.ReadFile(cfg.Api.KeyFile)
	if err != nil {
		logger.Error("Failed to read keys file: %s", err)
		os.Exit(1)
	}
	var keys api.KeyFile
	err = yaml.Unmarshal(yamlF, &keys)
	if err != nil {
		logger.Error("Failed to read keys file: %s", err)
		os.Exit(1)
	}
	rg := restgate.New("X-MptcpKit-Auth", "", restgate.Static, restgate.Config{
		Key:                []string{keys.Keys.Api},
		HTTPSProtectionOff: !cfg.Server.Https,
	})
	rgAdapter := func(c *gin.Context) {
		nextCalled := false
		nextAdapter := func(http.ResponseWriter, *http.Request) {
			nextCalled = true
			c.Next()
		}
		rg.ServeHTTP(c.Writer, c.Request, nextAdapter)
		if !nextCalled {
			c.AbortWithStatus(401)
		}
	}
	r.Use(rgAdapter)
	ctx := api.ApiContext{Config: &cfg, Log: logger}
	r.GET("/ip", ctx.IP)
	r.POST("/wan/update", ctx.WanIPsUpdate)

	r.GET("/ss/key", ctx.SSGetKey)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	if cfg.Server.Https {
		r.RunTLS(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port), cfg.Server.TLSCertFile, cfg.Server.TLSCertKey)
	} else {
		r.Run(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	}
}
