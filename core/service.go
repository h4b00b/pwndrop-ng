package core

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/kgretzky/pwndrop/log"

	"github.com/kgretzky/daemon"
	"github.com/otiai10/copy"
)

const INSTALL_DIR = "/usr/local/pwndrop-ng"
const EXEC_NAME = "pwndrop-ng"

type Service struct {
	Daemon daemon.Daemon
}

func (service *Service) Install() bool {
	var err error

	if runtime.GOOS == "windows" {
		log.Error("daemons disabled on windows")
		return false
	}

	if _, err = os.Stat(INSTALL_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(INSTALL_DIR, 0700); err != nil {
			log.Error("failed to create directory: %s", INSTALL_DIR)
			return false
		}
	}

	exec_path, _ := os.Executable()
	exec_dst := filepath.Join(INSTALL_DIR, EXEC_NAME)
	if err = copy.Copy(exec_path, exec_dst); err != nil {
		log.Error("failed to copy '%s' to: %s", exec_path, exec_dst)
		return false
	}
	log.Success("copied pwndrop executable to: %s", exec_dst)

	_, err = service.Daemon.Install(exec_dst)
	if err != nil {
		if err == daemon.ErrAlreadyInstalled {
			log.Info("service already installed")
		} else {
			log.Error("failed to install daemon: %s", err)
			return false
		}
	}

	log.Success("successfully installed daemon")
	return true
}

func (service *Service) Remove() bool {
	var err error

	if runtime.GOOS == "windows" {
		log.Error("daemons disabled on windows")
		return false
	}

	_, err = service.Daemon.Remove()
	if err != nil {
		log.Error("failed to install daemon: %s", err)
		return false
	}

	if _, err = os.Stat(INSTALL_DIR); err == nil {
		err = os.RemoveAll(INSTALL_DIR)
		if err != nil {
			log.Error("failed to delete directory: %s", INSTALL_DIR)
			return false
		}
	} else {
		log.Warning("directory not found: %s", INSTALL_DIR)
	}
	log.Success("deleted pwndrop directory")

	log.Success("successfully removed daemon")
	return true
}

func (service *Service) Start() bool {
	if runtime.GOOS == "windows" {
		log.Error("daemons disabled on windows")
		return false
	}

	_, err := service.Daemon.Start()
	if err != nil {
		if err == daemon.ErrAlreadyRunning {
			log.Info("daemon already running")
		} else {
			log.Error("failed to start daemon: %s", err)
			return false
		}
	}
	log.Success("pwndrop is running")
	return true
}

func (service *Service) Stop() bool {
	if runtime.GOOS == "windows" {
		log.Error("daemons disabled on windows")
		return false
	}

	_, err := service.Daemon.Stop()
	if err != nil {
		if err == daemon.ErrAlreadyStopped {
			log.Info("daemon already stopped")
		} else {
			log.Error("failed to stop daemon: %s", err)
			return false
		}
	}
	log.Success("pwndrop stopped")
	return true
}

func (service *Service) Status() bool {
	if runtime.GOOS == "windows" {
		log.Error("daemons disabled on windows")
		return false
	}

	status, err := service.Daemon.Status()
	if err != nil {
		log.Error("failed to get daemon status: %s", err)
		return false
	}
	log.Info("pwndrop status: %s", status)
	return true
}
