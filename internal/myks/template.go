package myks

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

//go:embed all:templates/*.yaml
var templateFs embed.FS

func writeSecretFile(secretName string, secretFilePath string, username string, password string) error {
	err := copyFileSystemToPath(templateFs, "templates", filepath.Join(os.TempDir(), "templates"))
	if err != nil {
		return err
	}
	res, err := runYttWithFilesAndStdin([]string{filepath.Join(os.TempDir(), "templates", "vendir_secret.ytt.yaml")}, nil, func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("render vendir secret yaml", name, args))
	}, "--data-value=secret_name="+secretName, "--data-value=username="+username, "--data-value=password="+password)
	if err != nil {
		return err
	}

	err = writeFile(secretFilePath, []byte(res.Stdout))
	if err != nil {
		return err
	}
	return nil
}
