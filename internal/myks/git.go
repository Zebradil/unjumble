package myks

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFiles map[string]string

// GetChangedFilesGit returns list of files changed since the baseRevision, if specified, and since the last commit
// TODO: exclude files that are outside of the myks root directory
func GetChangedFilesGit(baseRevision string) (ChangedFiles, error) {
	logFn := func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd("collect changed files for smart-mode", name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)

		} else {
			log.Debug().Msg(cmd)
		}
	}

	files := ChangedFiles{}
	if baseRevision != "" {
		result, err := runCmd("git", nil, []string{"diff", "--name-status", baseRevision}, logFn)
		if err != nil {
			return nil, err
		}
		for path, status := range convertToChangedFiles(result.Stdout) {
			files[path] = status
		}
	}

	result, err := runCmd("git", nil, []string{"status", "--porcelain", "--untracked-files"}, logFn)
	if err != nil {
		return nil, err
	}
	for path, status := range convertToChangedFiles(result.Stdout) {
		files[path] = status
	}

	return files, nil
}

func convertToChangedFiles(changes string) ChangedFiles {
	cfs := ChangedFiles{}
	expr := regexp.MustCompile(`^([A-Z]\t|[A-Z? ]{2} )(.*)$`)
	for _, str := range strings.Split(changes, "\n") {
		matches := expr.FindStringSubmatch(str)
		if len(matches) == 3 {
			// 1: the first character of the status, after trimming spaces and tabs
			// 2: the file path
			cfs[matches[2]] = strings.Trim(matches[1], " \t")[:1]
		}
	}
	return cfs
}
