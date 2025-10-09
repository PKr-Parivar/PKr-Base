package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/PKr-Parivar/PKr-Base/utils"
)

var LOGGER *log.Logger

func InitLogger() error {
	user_config_root_dir, err := utils.GetUserConfigRootDir()
	if err != nil {
		fmt.Println("Error while Getting User Config of Root Dir:", err)
		fmt.Println("Source: InitLogger()")
		return err
	}

	dir_name := filepath.Join(user_config_root_dir, "Logs")
	err = os.MkdirAll(dir_name, 0700)
	if err != nil {
		fmt.Printf("Error while Creating '%s' dir: %v\n", dir_name, err)
		fmt.Println("Source: InitUserLogger()")
		return err
	}

	log_file_path := filepath.Join(dir_name, "PKr-Base.log")
	log_file, err := os.OpenFile(log_file_path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		log.Printf("Error while Opening '%s' file: %v\n", log_file_path, err)
		log.Println("Source: InitUserLogger()")
		return err
	}

	LOGGER = log.New(log_file, "", log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}
