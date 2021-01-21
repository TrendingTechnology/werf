package inspector

import (
	"fmt"
	"path/filepath"
)

func (i Inspector) InspectConfigDockerfileContextAddFile(relPath string) error {
	if i.sharedContext.LooseGiterminism() {
		return nil
	}

	if isAccepted, err := i.giterminismConfig.IsConfigDockerfileContextAddFileAccepted(relPath); err != nil {
		return err
	} else if isAccepted {
		return nil
	}

	return NewExternalDependencyFoundError(fmt.Sprintf("contextAddFile '%s' not allowed", filepath.ToSlash(relPath)))
}
