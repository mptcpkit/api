package main

import (
	"fmt"
	"log/syslog"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/mattn/go-isatty"
	"github.com/tompinn23/mptcpkit/api"
	"github.com/unrolled/secure"

	"github.com/gin-gonic/gin"
	"github.com/pjebs/restgate"

	log "github.com/inconshreveable/log15"
)

// StatusCodeColor is the ANSI color for appropriately logging http status code to a terminal.
func StatusCodeLogLevel(p *gin.LogFormatterParams) log.Lvl {
	code := p.StatusCode

	switch {
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return log.LvlInfo
	case code >= http.StatusMultipleChoices && code < http.StatusInternalServerError:
		return log.LvlWarn
	default:
		return log.LvlError
	}
}

// LoggerWithConfig instance a Logger middleware with config.
func LoggerWithConfig(logger log.Logger) gin.HandlerFunc {

	// defaultLogFormatter is the default log format function Logger middleware uses.
	var defaultLogFormatter = func(isTerm bool, param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if isTerm {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		return fmt.Sprintf("%s %3d %s| %13v | %15s |%s %-7s %s %#v\n %s",
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			param.Path,
			param.ErrorMessage,
		)
	}

	isTerm := true

	out := gin.DefaultWriter

	if w, ok := out.(*os.File); !ok || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd())) {
		isTerm = false
	}

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log only when path is not being skipped
		param := gin.LogFormatterParams{
			Request: c.Request,
			Keys:    c.Keys,
		}

		// Stop timer
		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

		param.BodySize = c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path
		switch StatusCodeLogLevel(&param) {
		case log.LvlInfo:
			fmt.Println(defaultLogFormatter(isTerm, param))
			logger.Info(defaultLogFormatter(false, param))
		case log.LvlError:
			fmt.Println(defaultLogFormatter(isTerm, param))
			logger.Error(defaultLogFormatter(false, param))
		case log.LvlWarn:
			fmt.Println(defaultLogFormatter(isTerm, param))
			logger.Error(defaultLogFormatter(false, param))
		}

	}
}

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
	if syslog, err := log.SyslogHandler(syslog.LOG_DAEMON, "mptcpkit-api", log.LogfmtFormat()); err != nil {
		logger.SetHandler(log.MultiHandler(
			log.LvlFilterHandler(
				log.LvlError,
				log.Must.FileHandler(cfg.Server.LogFile, log.LogfmtFormat()),
			)))
	} else {
		logger.SetHandler(log.MultiHandler(
			log.StdoutHandler,
			log.LvlFilterHandler(
				log.LvlError,
				log.Must.FileHandler(cfg.Server.LogFile, log.LogfmtFormat()),
			),
			syslog,
		))
	}

	r := gin.New()
	r.Use(LoggerWithConfig(logger), gin.Recovery())
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
	logger.Info(fmt.Sprintf("Binding %s:%s", cfg.Server.Host, cfg.Server.Port))
	if cfg.Server.Https {
		logger.Info("HTTPs: true")
		r.RunTLS(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port), cfg.Server.TLSCertFile, cfg.Server.TLSCertKey)
	} else {
		logger.Warn("HTTPs: false")
		r.Run(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	}
}
