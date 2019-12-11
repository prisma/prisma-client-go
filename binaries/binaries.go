package binaries

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/prisma/photongo/binaries/platform"
	"github.com/prisma/photongo/logger"
)

// PrismaVersion is a hardcoded version of the Prisma CLI.
const PrismaVersion = "2.0.0-alpha.443"

// EngineVersion is a hardcoded version of the Prisma Engine.
// The versions can be found under https://github.com/prisma/prisma-engine/commits/master.
const EngineVersion = "4028eec09329a14692b13f06581329fddb7b2876"

const PrismaURL = "https://prisma-binaries-photongo.s3.eu-central-1.amazonaws.com/%s-%s-%s.gz"
const EngineURL = "https://prisma-builds.s3-eu-west-1.amazonaws.com/master/%s/%s/%s.gz"

// PrismaCLIName returns the local file path of where the CLI is located
func PrismaCLIName() string {
	variation := platform.Name()
	return fmt.Sprintf("prisma-cli-%s", variation)
}

// Fetch fetches the Prisma binaries needed for the generator to a given directory
func Fetch(toDir string) error {
	if toDir == "" {
		return fmt.Errorf("toDir must be provided")
	}

	if !strings.HasPrefix(toDir, "/") {
		return fmt.Errorf("toDir must be absolute")
	}

	// fetch the CLI
	cli := PrismaCLIName()
	to := path.Join(toDir, cli)
	url := fmt.Sprintf(PrismaURL, "prisma-cli", PrismaVersion, platform.Name())

	if _, err := os.Stat(to); os.IsNotExist(err) {
		logger.L.Printf("prisma cli doesn't exist, fetching...")

		if err := download(url, to); err != nil {
			return fmt.Errorf("could not download %s to %s: %w", url, to, err)
		}
	} else {
		logger.L.Printf("prisma cli is cached")
	}

	// fetch the engines
	engines := []string{
		"query-engine",
		"migration-engine",
		"introspection-engine",
	}

	binaryName := platform.BinaryNameWithSSL()

	for _, e := range engines {
		logger.L.Printf("checking %s...", e)

		to := path.Join(toDir, fmt.Sprintf("prisma-%s-%s", e, binaryName))

		urlName := e
		// the query-engine binary to on S3 is "prisma"
		if e == "query-engine" {
			urlName = "prisma"
		}
		url := fmt.Sprintf(EngineURL, EngineVersion, binaryName, urlName)

		if _, err := os.Stat(to); !os.IsNotExist(err) {
			logger.L.Printf("%s is cached", to)
			continue
		}

		logger.L.Printf("%s is missing, downloading...", e)

		if err := download(url, to); err != nil {
			return fmt.Errorf("could not download %s to %s: %w", url, to, err)
		}

		logger.L.Printf("verifying %s...", e)

		if err := verify(to); err != nil {
			return fmt.Errorf("could not run %s: %w", to, err)
		}

		logger.L.Printf("%s done", e)
	}

	return nil
}

func download(url string, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("could not get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		out, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("received code %d from %s: %+v", resp.StatusCode, url, string(out))
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create %s: %w", dest, err)
	}
	defer out.Close()

	if err := os.Chmod(dest, 0777); err != nil {
		return fmt.Errorf("could not chmod +x %s: %w", url, err)
	}

	g, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("could not create gzip reader: %w", err)
	}
	defer g.Close()

	if _, err := io.Copy(out, g); err != nil {
		return fmt.Errorf("could not copy %s: %w", url, err)
	}

	return nil
}

// verify that a given binary runs
// this is run as en extra function after download() because of https://github.com/golang/go/issues/22315
func verify(dest string) error {
	cmd := exec.Command(dest, "--help")

	if logger.Debug {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	}

	return cmd.Run()
}
