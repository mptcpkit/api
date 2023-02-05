package api

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/heartwilltell/log"
	"gopkg.in/yaml.v3"

	"github.com/gin-gonic/gin"
)

type KeyFile struct {
	Keys struct {
		Shadowsocks string `yaml:"shadowsocks"`
		Api         string `yaml:"api"`
	} `yaml:"keys"`
}

type ApiContext struct {
	Config *Configuration
	Log    log.Logger
}

func (c *ApiContext) IP(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"ip": ctx.ClientIP(),
	})
}

type WanIPs struct {
	IPs []string `json:"ips"`
}

func (c *ApiContext) WanIPsUpdate(ctx *gin.Context) {
	ips := WanIPs{}
	if err := ctx.BindJSON(&ips); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}
	collect_file := fmt.Sprintf("/tmp/wanips-update.%05d", rand.Int63n(1e16))
	cmd := exec.Command("sh", fmt.Sprintf("%s/wan-update", c.Config.Api.ScriptsDir), strings.Join(ips.IPs, " "))
	cmd.Dir = c.Config.Api.ScriptsDir
	cmd.Env = SanitizedEnvironment(c.Config)
	cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_DIR=%s", c.Config.Api.ScriptsDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("COLLECT_FILE=%s", collect_file))
	output, err := cmd.Output()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Log.Info("cmd: %s output is %s", cmd.Args, collect_file)
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"output": string(output),
	})
}

func (c *ApiContext) SSGetKey(ctx *gin.Context) {
	yamlF, err := os.ReadFile(c.Config.Api.KeyFile)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
	}
	var keys KeyFile
	err = yaml.Unmarshal(yamlF, &keys)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
	}
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"key":    keys.Keys.Shadowsocks,
	})
}
