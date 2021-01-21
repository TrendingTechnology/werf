package ls_tree

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/werf/logboek"

	"github.com/werf/werf/pkg/path_matcher"
)

type Result struct {
	repository                           *git.Repository
	repositoryFullFilepath               string
	tree                                 *object.Tree
	lsTreeEntries                        []*LsTreeEntry
	submodulesResults                    []*SubmoduleResult
	notInitializedSubmoduleFullFilepaths []string
}

type SubmoduleResult struct {
	*Result
}

type LsTreeEntry struct {
	FullFilepath string
	object.TreeEntry
}

func (r *Result) LsTree(ctx context.Context, pathMatcher path_matcher.PathMatcher) (*Result, error) {
	res := &Result{
		repository:                           r.repository,
		repositoryFullFilepath:               r.repositoryFullFilepath,
		tree:                                 r.tree,
		lsTreeEntries:                        []*LsTreeEntry{},
		submodulesResults:                    []*SubmoduleResult{},
		notInitializedSubmoduleFullFilepaths: []string{},
	}

	for _, lsTreeEntry := range r.lsTreeEntries {
		var entryLsTreeEntries []*LsTreeEntry
		var entrySubmodulesResults []*SubmoduleResult

		var err error
		if lsTreeEntry.FullFilepath == "" {
			isTreeMatched, shouldWalkThrough := pathMatcher.ProcessDirOrSubmodulePath(lsTreeEntry.FullFilepath)
			if isTreeMatched {
				if debugProcess() {
					logboek.Context(ctx).Debug().LogLn("Root tree was added")
				}
				entryLsTreeEntries = append(entryLsTreeEntries, lsTreeEntry)
			} else if shouldWalkThrough {
				if debugProcess() {
					logboek.Context(ctx).Debug().LogLn("Root tree was checking")
				}

				entryLsTreeEntries, entrySubmodulesResults, err = lsTreeWalk(ctx, r.repository, r.tree, r.repositoryFullFilepath, r.repositoryFullFilepath, pathMatcher)
				if err != nil {
					return nil, err
				}
			}
		} else {
			entryLsTreeEntries, entrySubmodulesResults, err = lsTreeEntryMatch(ctx, r.repository, r.tree, r.repositoryFullFilepath, r.repositoryFullFilepath, lsTreeEntry, pathMatcher)
		}

		res.lsTreeEntries = append(res.lsTreeEntries, entryLsTreeEntries...)
		res.submodulesResults = append(res.submodulesResults, entrySubmodulesResults...)
	}

	for _, submoduleResult := range r.submodulesResults {
		sr, err := submoduleResult.LsTree(ctx, pathMatcher)
		if err != nil {
			return nil, err
		}

		if !sr.IsEmpty() {
			res.submodulesResults = append(res.submodulesResults, &SubmoduleResult{sr})
		}
	}

	for _, submoduleFullFilepath := range r.notInitializedSubmoduleFullFilepaths {
		isMatched, shouldGoThrough := pathMatcher.ProcessDirOrSubmodulePath(submoduleFullFilepath)
		if isMatched || shouldGoThrough {
			res.notInitializedSubmoduleFullFilepaths = append(res.notInitializedSubmoduleFullFilepaths, submoduleFullFilepath)
		}
	}

	return res, nil
}

func (r *Result) Walk(f func(lsTreeEntry *LsTreeEntry) error) error {
	return r.walkWithResult(func(_ *Result, lsTreeEntry *LsTreeEntry) error {
		return f(lsTreeEntry)
	})
}

func (r *Result) walkWithResult(f func(r *Result, lsTreeEntry *LsTreeEntry) error) error {
	if err := r.lsTreeEntriesWalkWithExtra(f); err != nil {
		return err
	}

	sort.Slice(r.submodulesResults, func(i, j int) bool {
		return r.submodulesResults[i].repositoryFullFilepath < r.submodulesResults[j].repositoryFullFilepath
	})

	for _, submoduleResult := range r.submodulesResults {
		if err := submoduleResult.walkWithResult(f); err != nil {
			return err
		}
	}

	return nil
}

func (r *Result) Checksum(ctx context.Context) string {
	if r.IsEmpty() {
		return ""
	}

	h := sha256.New()

	_ = r.lsTreeEntriesWalk(func(lsTreeEntry *LsTreeEntry) error {
		h.Write([]byte(lsTreeEntry.Hash.String()))

		logFilepath := lsTreeEntry.FullFilepath
		if logFilepath == "" {
			logFilepath = "."
		}

		logboek.Context(ctx).Debug().LogF("Entry was added: %s -> %s\n", logFilepath, lsTreeEntry.Hash.String())

		return nil
	})

	sort.Strings(r.notInitializedSubmoduleFullFilepaths)
	for _, submoduleFullFilepath := range r.notInitializedSubmoduleFullFilepaths {
		checksumArg := fmt.Sprintf("-%s", filepath.ToSlash(submoduleFullFilepath))
		h.Write([]byte(checksumArg))
		logboek.Context(ctx).Debug().LogF("Not initialized submodule was added: %s -> %s\n", submoduleFullFilepath, checksumArg)
	}

	sort.Slice(r.submodulesResults, func(i, j int) bool {
		return r.submodulesResults[i].repositoryFullFilepath < r.submodulesResults[j].repositoryFullFilepath
	})

	for _, submoduleResult := range r.submodulesResults {
		var submoduleChecksum string
		if !submoduleResult.IsEmpty() {
			logboek.Context(ctx).Debug().LogOptionalLn()
			blockMsg := fmt.Sprintf("submodule %s", submoduleResult.repositoryFullFilepath)
			logboek.Context(ctx).Debug().LogBlock(blockMsg).Do(func() {
				submoduleChecksum = submoduleResult.Checksum(ctx)
				logboek.Context(ctx).Debug().LogLn()
				logboek.Context(ctx).Debug().LogLn(submoduleChecksum)
			})
		}

		if submoduleChecksum != "" {
			h.Write([]byte(submoduleChecksum))
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *Result) IsEmpty() bool {
	return len(r.lsTreeEntries) == 0 && len(r.submodulesResults) == 0 && len(r.notInitializedSubmoduleFullFilepaths) == 0
}

func (r *Result) lsTreeEntriesWalk(f func(entry *LsTreeEntry) error) error {
	return r.lsTreeEntriesWalkWithExtra(func(_ *Result, entry *LsTreeEntry) error {
		return f(entry)
	})
}

func (r *Result) lsTreeEntriesWalkWithExtra(f func(r *Result, entry *LsTreeEntry) error) error {
	sort.Slice(r.lsTreeEntries, func(i, j int) bool {
		return r.lsTreeEntries[i].FullFilepath < r.lsTreeEntries[j].FullFilepath
	})

	for _, lsTreeEntry := range r.lsTreeEntries {
		if err := f(r, lsTreeEntry); err != nil {
			return err
		}
	}

	return nil
}

func (r *Result) LsTreeEntry(resolvedRelPath string) *LsTreeEntry {
	var lsTreeEntry *LsTreeEntry
	_ = r.Walk(func(entry *LsTreeEntry) error {
		if filepath.ToSlash(entry.FullFilepath) == filepath.ToSlash(resolvedRelPath) {
			lsTreeEntry = entry
		}

		return nil
	})

	if lsTreeEntry == nil {
		lsTreeEntry = &LsTreeEntry{
			FullFilepath: resolvedRelPath,
			TreeEntry: object.TreeEntry{
				Name: resolvedRelPath,
				Mode: filemode.Empty,
				Hash: plumbing.Hash{},
			},
		}
	}

	return lsTreeEntry
}

func (r *Result) LsTreeEntryContent(relPath string) ([]byte, error) {
	var entryResult *Result
	var entry *LsTreeEntry

	_ = r.walkWithResult(func(er *Result, e *LsTreeEntry) error {
		if filepath.ToSlash(e.FullFilepath) == filepath.ToSlash(relPath) {
			entryResult = er
			entry = e
		}

		return nil
	})

	if entry == nil {
		return nil, fmt.Errorf("unable to get tree entry %s", relPath)
	}

	if entryResult == nil {
		panic("unexpected condition")
	}

	obj, err := entryResult.repository.BlobObject(entry.Hash)
	if err != nil {
		return nil, err
	}

	f, err := obj.Reader()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("unable to read tree entry content %s", relPath)
	}

	return data, err
}
