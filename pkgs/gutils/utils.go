package gutils

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/moqsien/goutils/pkgs/gtui"
)

func PathIsExist(path string) (bool, error) {
	_, _err := os.Stat(path)
	if _err == nil {
		return true, nil
	}
	if os.IsNotExist(_err) {
		return false, nil
	}
	return false, _err
}

func MakeDirs(dirs ...string) {
	for _, d := range dirs {
		if ok, _ := PathIsExist(d); !ok {
			if err := os.MkdirAll(d, os.ModePerm); err != nil {
				fmt.Println("mkdir failed: ", err)
			}
		}
	}
}

func Closeq(v any) {
	if c, ok := v.(io.Closer); ok {
		silently(c.Close())
	}
}

func silently(_ ...any) {}

func CopyFile(src, dst string) (written int64, err error) {
	srcFile, err := os.Open(src)

	if err != nil {
		gtui.PrintError(fmt.Sprintf("Cannot open file: %+v", err))
		return
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		gtui.PrintError(fmt.Sprintf("Cannot open file: %+v", err))
		return
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

func CheckSum(fpath, cType, cSum string) (r bool) {
	if cSum != ComputeSum(fpath, cType) {
		gtui.PrintError("Checksum failed.")
		return
	}
	gtui.PrintSuccess("Checksum succeeded.")
	return true
}

func ComputeSum(fpath, sumType string) (sumStr string) {
	f, err := os.Open(fpath)
	if err != nil {
		gtui.PrintError(fmt.Sprintf("Open file failed: %+v", err))
		return
	}
	defer f.Close()

	var h hash.Hash
	switch strings.ToLower(sumType) {
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New()
	case "sha512":
		h = sha512.New()
	default:
		gtui.PrintError(fmt.Sprintf("[Crypto] %s is not supported.", sumType))
		return
	}

	if _, err = io.Copy(h, f); err != nil {
		gtui.PrintError(fmt.Sprintf("Copy file failed: %+v", err))
		return
	}

	sumStr = hex.EncodeToString(h.Sum(nil))
	return
}

func VerifyUrls(rawUrl string) (r bool) {
	r = true
	_, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		r = false
		return
	}
	url, err := url.Parse(rawUrl)
	if err != nil || url.Scheme == "" || url.Host == "" {
		r = false
		return
	}
	return
}

const (
	Win     string = "win"
	Zsh     string = "zsh"
	Bash    string = "bash"
	Windows string = "windows"
	Darwin  string = "darwin"
	Linux   string = "linux"
)

func GetHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func GetShell() (shell string) {
	if runtime.GOOS == Windows {
		return Win
	}
	s := os.Getenv("SHELL")
	if strings.Contains(s, "zsh") {
		return Zsh
	}
	return Bash
}

func GetShellRcFile() (rc string) {
	shell := GetShell()
	switch shell {
	case Zsh:
		rc = filepath.Join(GetHomeDir(), ".zshrc")
	case Bash:
		rc = filepath.Join(GetHomeDir(), ".bashrc")
	default:
		rc = Win
	}
	return
}

func FlushPathEnvForUnix() (err error) {
	if runtime.GOOS != Windows {
		err = exec.Command("source", GetShellRcFile()).Run()
	}
	return
}

func ExecuteSysCommand(collectOutput bool, args ...string) (*bytes.Buffer, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == Windows {
		args = append([]string{"/c"}, args...)
		cmd = exec.Command("cmd", args...)
	} else {
		FlushPathEnvForUnix()
		cmd = exec.Command(args[0], args[1:]...)
	}
	cmd.Env = os.Environ()
	var output bytes.Buffer
	if collectOutput {
		cmd.Stdout = &output
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return &output, cmd.Run()
}
