//+build mage

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/pkg/errors"
)

// TODO: Enable this once there is a main package.
//// Build builds the program.
//func Build(ctx context.Context) error {
//	args := []string{"build", "-ldflags=-s -w"}
//	if runtime.GOOS == "linux" {
//		args = append(args, "-buildmode=pie")
//	}
//	cmd := exec.CommandContext(ctx, "go", args...)
//	// To minimize OS library dependence.
//	vars := positiveEnvs{"CGO_ENABLED": "0"}
//	_, err := runCommand(cmd, mg.Verbose(), vars)
//	return err
//}

// Test runs all of the basic tests.
func Test(ctx context.Context) error {
	args := []string{"test"}
	if mg.Verbose() {
		args = append(args, "-v")
	}
	args = append(args, "./...")
	cmd := exec.CommandContext(ctx, "go", args...)
	_, err := runCommand(cmd, mg.Verbose())
	return err
}

// TestRepeat runs all of the basic tests multiple times with the race detector
// enabled to better determine if the the tests are consistent and race free.
func TestRepeat(ctx context.Context) error {
	args := []string{"test"}
	if mg.Verbose() {
		args = append(args, "-v", "-race", "-count=10")
	}
	if env := os.Getenv("CPU_PROFILE"); env != "" {
		args = append(args, fmt.Sprintf("-cpuprofile=%s", env))
	}
	if env := os.Getenv("TEST_PKG"); env != "" {
		args = append(args, env)
	} else {
		args = append(args, "./...")
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	_, err := runCommand(cmd, mg.Verbose())
	return err
}

// TestIntegration runs integration tests.
func TestIntegration(ctx context.Context) error {
	args := []string{"test", "-tags=integration"}
	if env := os.Getenv("TEST_RUN"); env != "" {
		args = append(args, fmt.Sprintf("-run=%s", env))
	}
	if mg.Verbose() {
		args = append(args, "-v")
	}
	args = append(args, "./test/integration/...")
	cmd := exec.CommandContext(ctx, "go", args...)
	vars := positiveEnvs{"CGO_ENABLED": "0"}
	_, err := runCommand(cmd, mg.Verbose(), vars)
	return err
}

// Lint runs the linters.
//
// Note that the linters emitting warnings will not be considered a failure.
func Lint(ctx context.Context) error {
	mg.SerialCtxDeps(ctx, vet, golint)
	return nil
}

// vet runs go vet.
func vet(ctx context.Context) error {
	packages, err := getNonVendorPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get package list")
	}

	var args = append([]string{"vet", "-tags=integration"}, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	_, err = runCommand(cmd, mg.Verbose())
	return err
}

// golint runs golint.
func golint(ctx context.Context) error {
	_, err := exec.LookPath("golint")
	if err != nil {
		return errors.Wrap(err, "unable to run golint")
	}

	packages, err := getNonVendorPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get package list")
	}

	// Build tags not currently supported for golint, so integration files
	// will be skipped for now.
	args := []string{"-min_confidence=0.3"}
	args = append(args, packages...)
	cmd := exec.CommandContext(ctx, "golint", args...)
	_, err = runCommand(cmd, mg.Verbose())
	return err
}

// getNonVendorPackages gets the package paths for all of the packages that are
// not under /vendor.
//
// This is only needed due to not all tools supported /vendor exclusion with
// "./...".
func getNonVendorPackages(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "./...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Errorf("unable to get non-vendored packages: %+v\nOutput:\n%s", err, string(out))
	}

	var packages []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		packages = append(packages, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, errors.Wrap(err, "unable to get non-vendored packages")
	}

	return packages, nil
}

type (
	// noUseEnvs indicates that the environment variables from this process should
	// not be used.
	noUseEnvs struct{}
	// positiveEnvs are environment variables that should be added to the set of
	// environment variables.
	positiveEnvs map[string]string
	// negativeEnvs are environment variables that should be removed from the set of
	// environment variables.
	negativeEnvs []string
)

func removeEnvVar(envs *[]string, key string) {
	for i, v := range *envs {
		split := strings.SplitN(v, "=", 2)
		if split[0] == key {
			*envs = append((*envs)[:i], (*envs)[i+1:]...)
			return
		}
	}
}

func runCommand(cmd *exec.Cmd, useStdOutput bool, options ...interface{}) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	var w io.Writer
	if useStdOutput {
		w = io.MultiWriter(buf, os.Stdout)
	} else {
		w = buf
	}
	cmd.Stdout = w
	cmd.Stderr = w

	var envs []string
	if len(options) > 0 {
		if _, ok := options[0].(noUseEnvs); !ok {
			envs = make([]string, len(os.Environ()))
			copy(envs, os.Environ())
		}
	}

	for _, v := range options {
		switch v.(type) {
		case positiveEnvs:
			pe := v.(positiveEnvs)
			for k, v := range pe {
				envs = append(envs, fmt.Sprintf("%s=%s", k, v))
			}
		case negativeEnvs:
			ne := v.(negativeEnvs)
			for _, k := range ne {
				removeEnvVar(&envs, k)
			}
		}
	}
	cmd.Env = envs

	err := cmd.Run()
	return buf, err
}
