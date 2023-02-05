package api

//#include <paths.h>
import "C"
import (
	"fmt"
	"os"
)

func SanitizedEnvironment(c *Configuration) []string {
	var env []string
	env = append(env, fmt.Sprintf("PATH=%s", C._PATH_STDPATH))
	env = append(env, fmt.Sprintf("IFS= \t\n"))
	env = append(env, fmt.Sprintf("TZ=%s", os.Getenv("TZ")))
	if c.Api.DryRun {
		env = append(env, "MPTCPKIT_DRYRUN=1")
	}
	return env
}
