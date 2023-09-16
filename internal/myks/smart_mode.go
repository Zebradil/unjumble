package myks

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func (g *Globe) getGlobalLibDirExpr() string {
	return "^" + g.YttLibraryDirName + "/.*$"
}

func (g *Globe) getGlobalYttDirExpr() string {
	return "^" + g.EnvironmentBaseDir + "/_env/" + g.YttStepDirName + "/.*$"
}

func (g *Globe) getGlobalEnvExpr() string {
	return "^" + g.EnvironmentBaseDir + "/" + g.EnvironmentDataFileName + "$"
}

func (g *Globe) getPrototypeExpr() string {
	subDirs := [...]string{g.YttStepDirName, "helm", "vendir", g.YttPkgStepDirName, g.ArgoCDDataDirName}
	return "^" + g.PrototypesDir + "/(?:.*?/)?(.*?)/(?:" + strings.Join(subDirs[:], "|") + ")/.*$"
}

func (g *Globe) getPrototypeDataFileExpr() string {
	return "^" + g.PrototypesDir + "/(?:.*?/)?(.*?)/" + g.ApplicationDataFileName + "$"
}

func (g *Globe) getEnvsExpr() string {
	return "^(" + g.EnvironmentBaseDir + "/.+)/" + g.EnvironmentDataFileName + "$"
}

func (g *Globe) getAppsExpr() string {
	return "^(" + g.EnvironmentBaseDir + "/.*?)/_apps/(.*?)/.*$"
}

func (g *Globe) DetectChangedEnvsAndApps() ([]string, []string, error) {
	g.collectEnvironments(nil)

	err := process(0, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.initEnvData()
	})
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to collect environments"))
		return nil, nil, err
	}

	curRev, err := getDiffRevision(g.MainBranchName)
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to get current revision"))
		return nil, nil, err
	}
	changedFiles, err := getChangedFiles(curRev)
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to get diff"))
		return nil, nil, err
	}
	envs, apps := g.runSmartMode(changedFiles)
	log.Info().Msg(g.Msg(fmt.Sprintf("Smart mode detected changes in environments: %v, applications: %v", envs, apps)))

	missingApps, err := g.MissingApplications()
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to get missing applications"))
		return nil, nil, err
	}

	apps = append(apps, missingApps...)

	return envs, apps, nil
}

// find apps that are missing from rendered folder
func (g *Globe) MissingApplications() ([]string, error) {
	missingApps := []string{}
	for name, e := range g.environments {
		missing, err := e.MissingApplications()
		if err != nil {
			log.Err(err).Msg(g.Msg(fmt.Sprintf("Failed to get missing applications for environment %s", name)))
			return nil, err
		}

		missingApps = append(missingApps, missing...)
	}

	return missingApps, nil
}

func (g *Globe) runSmartMode(changedFiles ChangedFiles) ([]string, []string) {
	allChangedFilePaths := maps.Keys(changedFiles)
	allDeletions := maps.Keys(filterMap[string, string](changedFiles, func(k, v string) bool {
		return v == "D"
	}))
	allChangedFilesExceptDeletions := maps.Keys(filterMap[string, string](changedFiles, func(k, v string) bool {
		return v != "D"
	}))

	if g.checkGlobalConfigChanged(allChangedFilePaths) {
		return nil, nil
	}
	modifiedEnvs := g.getModifiedEnvs(allChangedFilesExceptDeletions)
	modifiedEnvsFromApp, modifiedApps := g.getModifiedApps(allChangedFilePaths, g.getModifiedEnvs(allDeletions))
	modifiedPrototypes := g.getModifiedPrototypes(allChangedFilePaths)
	modifiedEnvsFromPrototype, modifiedAppsFromPrototype := g.findPrototypeUsage(modifiedPrototypes)

	// Once envs have been modified globally, we can no longer render individual apps, since we don't know which apps are affected.
	// This goes for editing of env-data.ytt.yaml, global ytt files as well as manifests.
	if len(modifiedEnvs) > 0 {
		envs := append(modifiedEnvs, modifiedEnvsFromPrototype...)
		envs = append(envs, modifiedEnvsFromApp...)
		envs = removeDuplicates(envs)
		sort.Strings(envs)
		return envs, nil
	} else {
		envs := removeDuplicates(append(modifiedEnvsFromPrototype, modifiedEnvsFromApp...))
		sort.Strings(envs)
		apps := removeDuplicates(append(modifiedApps, modifiedAppsFromPrototype...))
		sort.Strings(apps)
		return envs, apps
	}
}

func (g *Globe) findPrototypeUsage(prototypes []string) ([]string, []string) {
	var envs []string
	var apps []string
	for _, prototype := range prototypes {
		for envPath, env := range g.environments {
			for appProto, appName := range env.foundApplications {
				if appProto == prototype || strings.HasSuffix(appProto, "/"+prototype) {
					envs = append(envs, envPath)
					apps = append(apps, appName)
				}
			}
		}
	}
	return removeDuplicates(envs), removeDuplicates(apps)
}

func (g *Globe) checkGlobalConfigChanged(changedFiles []string) bool {
	return checkFileChanged(changedFiles, g.getGlobalLibDirExpr(), g.getGlobalYttDirExpr(), g.getGlobalEnvExpr())
}

func (g *Globe) getModifiedPrototypes(changedFiles []string) []string {
	changes, _ := getChanges(changedFiles, g.getPrototypeDataFileExpr(), g.getPrototypeExpr())
	return changes
}

func (g *Globe) getModifiedApps(changedFiles []string, deletedEnvs []string) ([]string, []string) {
	envs, apps := getChanges(changedFiles, g.getAppsExpr())
	return filterDeletedEnvs(envs, apps, deletedEnvs)
}

func (g *Globe) getModifiedEnvs(changedFiles []string) []string {
	modifiedEnvs, _ := getChanges(changedFiles, g.getEnvsExpr())
	return removeSubPaths(modifiedEnvs)
}

func checkFileChanged(changedFiles []string, regExps ...string) bool {
	for _, expr := range regExps {
		changes, _ := getChanges(changedFiles, expr)
		if len(changes) > 0 {
			return true
		}
	}
	return false
}

func getChanges(changedFilePaths []string, regExps ...string) ([]string, []string) {
	var matches1 []string
	var matches2 []string
	for _, expr := range regExps {
		for _, line := range changedFilePaths {
			expr := regexp.MustCompile(expr)
			matches := expr.FindStringSubmatch(line)
			if matches != nil {
				if len(matches) == 1 {
					matches1 = append(matches1, matches[0])
				} else if len(matches) == 2 {
					matches1 = append(matches1, matches[1])
				} else {
					matches1 = append(matches1, matches[1])
					matches2 = append(matches2, matches[2])
				}
			}
		}
	}
	return matches1, matches2
}

func filterDeletedEnvs(envs []string, apps []string, deletedEnvs []string) ([]string, []string) {
	var resultEnvs []string
	var resultApps []string
	for i, env := range envs {
		if !slices.Contains(deletedEnvs, env) {
			resultEnvs = append(resultEnvs, env)
			resultApps = append(resultApps, apps[i])
		}
	}

	return resultEnvs, resultApps
}
