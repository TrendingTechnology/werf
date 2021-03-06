package secret

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/werf/werf/pkg/secret"
	"github.com/werf/werf/pkg/util"
	"github.com/werf/werf/pkg/werf"
)

type Manager interface {
	secret.Secret

	EncryptYamlData(data []byte) ([]byte, error)
	DecryptYamlData(encodedData []byte) ([]byte, error)
}

func GenerateSecretKey() ([]byte, error) {
	return secret.GenerateAexSecretKey()
}

func GetManager(workingDir string) (Manager, error) {
	var m Manager
	var key []byte
	var err error

	key, err = GetSecretKey(workingDir)
	if err != nil {
		return nil, err
	}

	m, err = NewManager(key)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func GetSecretKey(workingDir string) ([]byte, error) {
	var secretKey []byte
	var werfSecretKeyPaths []string
	var notFoundIn []string

	secretKey = []byte(os.Getenv("WERF_SECRET_KEY"))
	if len(secretKey) == 0 {
		notFoundIn = append(notFoundIn, "$WERF_SECRET_KEY")

		var werfSecretKeyPath string

		if workingDir != "" {
			if defaultWerfSecretKeyPath, err := filepath.Abs(filepath.Join(workingDir, ".werf_secret_key")); err != nil {
				return nil, err
			} else {
				werfSecretKeyPaths = append(werfSecretKeyPaths, defaultWerfSecretKeyPath)
			}
		}

		werfSecretKeyPaths = append(werfSecretKeyPaths, filepath.Join(werf.GetHomeDir(), "global_secret_key"))

		for _, path := range werfSecretKeyPaths {
			exist, err := util.FileExists(path)
			if err != nil {
				return nil, err
			}

			if exist {
				werfSecretKeyPath = path
				break
			} else {
				notFoundIn = append(notFoundIn, path)
			}
		}

		if werfSecretKeyPath != "" {
			data, err := ioutil.ReadFile(werfSecretKeyPath)
			if err != nil {
				return nil, err
			}

			secretKey = []byte(strings.TrimSpace(string(data)))
		}
	}

	if len(secretKey) == 0 {
		return nil, fmt.Errorf("encryption key not found in: %q", strings.Join(notFoundIn, "', '"))
	}

	return secretKey, nil
}

func NewManager(key []byte) (Manager, error) {
	ss, err := secret.NewSecret(key)
	if err != nil {
		return nil, fmt.Errorf("check encryption key: %s", err)
	}

	return newBaseManager(ss)
}

func NewSafeManager() (Manager, error) {
	return newBaseManager(nil)
}
