// +build integration

package sshserver

import (
	"bufio"
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	errors2 "errors"

	"github.com/pkg/errors"
)

const (
	integrationImageName = "ex-integration"
	intDockerfile        = `FROM ubuntu:xenial
RUN apt-get update && apt-get install -y openssh-server
RUN adduser --disabled-password --gecos "" test && echo "test:password123" | chpasswd
`
)

var (
	getImageArgs   = []string{"images"}
	buildImageArgs = []string{"build", "-t", integrationImageName, "."}
)

// getDockerCmd gets the command needed to run Docker.
//
// Will return a slice with at least one element in it.
func getDockerCmd() []string {
	return []string{"docker"}
}

var (
	errNoImage            = errors2.New("no matching image found")
	errTooManyImagesFound = errors2.New("too many images found")
)

// checkImageExists checks to see if the image with the given name exists.
//
// There must be only one image with the given name.
func checkImageExists(ctx context.Context, name string) error {
	// Would check $PATH to see if docker exists in it here, but that would not
	// technically be correct. In the case where sudo (or something similar) is
	// used, the $PATH may change and make $PATH checking incorrect.

	dockerCmd := getDockerCmd()
	args := append(getImageArgs, name)
	cmd := exec.CommandContext(ctx, dockerCmd[0], append(dockerCmd[1:], args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Errorf("unable to get Docker images: %+v, (Output: %s)", err, string(out))
	}

	var lineCount int
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		lineCount++
	}
	if err = scanner.Err(); err != nil {
		return errors.Wrap(err, "unable to process Docker images output")
	}

	if lineCount <= 1 {
		return errors.WithStack(errNoImage)
	} else if lineCount == 2 {
		return nil
	}
	return errors.WithStack(errTooManyImagesFound)
}

func createIntImage(ctx context.Context) error {
	dirName, err := ioutil.TempDir("", "ex-int")
	if err != nil {
		return errors.Wrap(err, "unable to create integration image")
	}
	defer os.RemoveAll(dirName)

	fPath := filepath.Join(dirName, "Dockerfile")
	err = ioutil.WriteFile(fPath, []byte(intDockerfile), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "unable to create Dockerfile for integration image")
	}

	oldPWD, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "unable to get current directory for building")
	}
	err = os.Chdir(dirName)
	if err != nil {
		return errors.Wrap(err, "unable to change to directory of Dockerfile for building")
	}
	defer os.Chdir(oldPWD)

	dockerCmd := getDockerCmd()
	cmd := exec.CommandContext(ctx, dockerCmd[0], append(dockerCmd[1:], buildImageArgs...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Errorf("unable to get Docker images: %+v, (Output: %s)", err, string(out))
	}

	err = checkImageExists(ctx, integrationImageName)
	if err != nil {
		return errors.Wrap(err, "unexpected integration image creation failure")
	}

	return nil
}

func ensureIntImageExists(ctx context.Context) error {
	err := checkImageExists(ctx, integrationImageName)
	if err != nil {
		if errors.Cause(err) == errNoImage {
			err = createIntImage(ctx)
			if err != nil {
				return errors.Wrap(err, "unable to ensure integration image exists")
			}
			return nil
		}
		return errors.Wrap(err, "unable to ensure integration image exists")
	}

	return nil
}
