	"os"
func debugPatchParser() bool {
	return os.Getenv("WERF_TRUE_GIT_DEBUG_PATCH_PARSER") == "1"
}

	if debugPatchParser() {
		oldState := p.state
		fmt.Printf("TRUE_GIT parse diff line: state=%#v line=%#v\n", oldState, line)
		defer func() {
			fmt.Printf("TRUE_GIT parse diff line: state change: %#v => %#v\n", oldState, p.state)
		}()
	}

		if strings.HasPrefix(line, "diff --git ") {
			return p.handleDiffBegin(line)
		}
		if strings.HasPrefix(line, "Submodule ") {
			return p.handleSubmoduleLine(line)
		}