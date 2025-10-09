package filetracker

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/PKr-Parivar/PKr-Base/config"
)

func addFilesToZip(writer *zip.Writer, dir_path string, relativepath string) error {
	files, err := ioutil.ReadDir(dir_path)
	if err != nil {
		fmt.Println("Error while Reading Dir:", err)
		fmt.Println("Source: addFilesToZip()")
		return err
	}

	for _, file := range files {
		if file.Name() == ".PKr" || file.Name() == "PKr-Base.exe" || file.Name() == "PKr-Cli.exe" || file.Name() == "PKr-Base" || file.Name() == "PKr-Cli" {
			continue
		} else if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(dir_path, file.Name()))
			if err != nil {
				fmt.Println("Error while Reading File:", err)
				fmt.Println("Source: addFilesToZip()")
				return err
			}

			file, err := writer.Create(filepath.Join(relativepath, file.Name()))
			if err != nil {
				fmt.Println("Error while Creating Entry in Zip File:", err)
				fmt.Println("Source: addFilesToZip()")
				return err
			}
			file.Write(content)
		} else if file.IsDir() {
			new_dir_path := filepath.Join(dir_path, file.Name()) + string(os.PathSeparator)
			new_rel_path := filepath.Join(relativepath, file.Name()) + string(os.PathSeparator)

			addFilesToZip(writer, new_dir_path, new_rel_path)
		}
	}
	return nil
}

func ZipData(workspace_path string, destination_path string, zip_file_name string) error {
	zip_file_name = zip_file_name + ".zip"
	full_zip_path := filepath.Join(destination_path, zip_file_name)

	// Ensure the destination directory exists
	err := os.MkdirAll(destination_path, 0700)
	if err != nil {
		fmt.Println("Error creating destination directory:", err)
		fmt.Println("Source: ZipData()")
		return err
	}
	zip_file, err := os.Create(full_zip_path)
	if err != nil {
		fmt.Println("Error while Creating Zip File:", err)
		fmt.Println("Source: ZipData()")
		return err
	}

	writer := zip.NewWriter(zip_file)
	addFilesToZip(writer, workspace_path, "")

	if err = writer.Close(); err != nil {
		fmt.Println("Error while Closing zip writer:", err)
		fmt.Println("Source: ZipData()")
		return err
	}
	zip_file.Close()
	return nil
}

func UnzipData(src, dest string) error {
	fmt.Printf("Unzipping Files: %s\n\t to %s\n", src, dest)
	zipper, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zipper.Close()
	total_files := 0
	for count, file := range zipper.File {
		if file.FileInfo().IsDir() {
			continue
		}

		var temp_file_name string
		if runtime.GOOS == "windows" {
			temp_file_name = strings.ReplaceAll(file.Name, "/", "\\")
		} else {
			temp_file_name = strings.ReplaceAll(file.Name, "\\", "/")
		}

		abs_path := filepath.Join(dest, temp_file_name)
		dir, _ := filepath.Split(abs_path)

		if dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return err
			}
		}
		unzip_file, err := os.Create(abs_path)
		if err != nil {
			return err
		}

		content, err := file.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(unzip_file, content)
		if err != nil {
			return err
		}

		// Close both after copy
		unzip_file.Close()
		content.Close()

		total_files += 1
		fmt.Printf("%d] File: %s\n", count, temp_file_name)
	}
	fmt.Printf("\nTotal Files Recieved: %d\n", total_files)
	return nil
}

func returnZipFileObj(zip_file_reader *zip.ReadCloser, search_file_name string) *zip.File {
	for _, file := range zip_file_reader.File {
		if file.Name == search_file_name {
			return file
		}
	}
	return nil
}

func ZipUpdates(changes []config.FileChange, src_path string, dst_path string) (err error) {
	dst_dir, _ := filepath.Split(dst_path)
	if err = os.Mkdir(dst_dir, 0700); err != nil {
		fmt.Println("Error Could not Create the Dir:", err)
		fmt.Println("Source: ZipUpdates()")
		return err
	}

	// Open Src Zip File
	src_zip_file, err := zip.OpenReader(src_path)
	if err != nil {
		fmt.Println("Error while Opening Source Zip File:", err)
		fmt.Println("Source: ZipUpdates()")
		return err
	}
	defer src_zip_file.Close()

	// Create Dest Zip File
	dst_zip_file, err := os.Create(dst_path)
	if err != nil {
		fmt.Printf("Error Could Not Create File %v: %v\n", dst_path, err)
		fmt.Println("Source: ZipUpdates()")
		return err
	}
	defer dst_zip_file.Close()

	// Dest Zip Writer
	writer := zip.NewWriter(dst_zip_file)
	defer writer.Close()

	for _, change := range changes {
		if change.Type != "Updated" {
			continue
		}

		zip_file_obj := returnZipFileObj(src_zip_file, change.FilePath)
		if zip_file_obj == nil {
			fmt.Println("Error while Getting Zip File Obj:", filepath.Join(src_path, change.FilePath), "is nil")
			fmt.Println("Source: ZipUpdates()")
			return
		}

		zip_file_obj_reader, err := zip_file_obj.Open()
		if err != nil {
			return err
		}
		defer zip_file_obj_reader.Close()

		new_file, err := writer.Create(zip_file_obj.Name)
		if err != nil {
			return err
		}

		// Copy the contents
		_, err = io.Copy(new_file, zip_file_obj_reader)
		if err != nil {
			return err
		}
	}
	return nil
}
