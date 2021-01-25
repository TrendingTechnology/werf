package inspector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/werf/logboek"

	"github.com/werf/werf/pkg/git_repo/status"
	"github.com/werf/werf/pkg/giterminism_manager/errors"
	"github.com/werf/werf/pkg/path_matcher"
)

func (i Inspector) InspectBuildContextFiles(ctx context.Context, matcher path_matcher.PathMatcher) error {
	logProcess := logboek.Context(ctx).Debug().LogProcess("status (%s)", matcher.String())
	logProcess.Start()
	result, err := i.sharedOptions.LocalGitRepo().Status(ctx, matcher)
	if err != nil {
		logProcess.Fail()
		return err
	} else {
		logProcess.End()
	}

	filePathList := result.FilePathList(status.FilterOptions{WorktreeOnly: i.sharedOptions.Dev()})
	if len(filePathList) != 0 {
		return errors.NewError(fmt.Sprintf("the following files changes must be committed:\n\n%s", prepareListOfFilesString(filePathList)))
	}

	return nil
}

func prepareListOfFilesString(paths []string) string {
	var result string
	for _, path := range paths {
		result += " - " + filepath.ToSlash(path) + "\n"
	}

	return strings.TrimSuffix(result, "\n")
}
