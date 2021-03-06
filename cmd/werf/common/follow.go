package common

import (
	"context"
	"time"

	"github.com/werf/logboek"
	"github.com/werf/logboek/pkg/style"
	"github.com/werf/logboek/pkg/types"
)

func FollowGitHead(ctx context.Context, cmdData *CmdData, taskFunc func(ctx context.Context) error) error {
	workTree, err := GetGitWorkTree(cmdData)
	if err != nil {
		return err
	}

	var savedHeadCommit string
	iterFunc := func() error {
		l, err := OpenLocalGitRepo(workTree)
		if err != nil {
			return err
		}

		currentHeadCommit, err := l.HeadCommit(ctx)
		if err != nil {
			return err
		}

		if savedHeadCommit != currentHeadCommit {
			savedHeadCommit = currentHeadCommit

			if err := logboek.Context(ctx).LogProcess("Commit %s", savedHeadCommit).
				Options(func(options types.LogProcessOptionsInterface) {
					options.Style(style.Highlight())
				}).
				DoError(func() error {
					return taskFunc(ctx)
				}); err != nil {
				return err
			}

			logboek.Context(ctx).LogLn("Waiting for new commit ...")
			logboek.Context(ctx).LogOptionalLn()
		} else {
			time.Sleep(1 * time.Second)
		}

		return nil
	}

	if err := iterFunc(); err != nil {
		return err
	}

	for {
		if err := iterFunc(); err != nil {
			logboek.Context(ctx).Warn().LogLn(err)
		}
	}
}
