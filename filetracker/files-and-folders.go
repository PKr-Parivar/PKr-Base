package filetracker

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/PKr-Parivar/PKr-Base/encrypt"
)

// Delete files and folders in the Workspace Except: /.PKr , PKr-base.exe, PKr-cli.exe
func CleanFilesFromWorkspace(workspace_path string) error {
	files, err := ioutil.ReadDir(workspace_path)
	if err != nil {
		fmt.Println("Error while Cleaning Files from Workspace:", err)
		fmt.Println("Source: CleanFilesFromWorkspace()")
		return err
	}

	for _, file := range files {
		if file.Name() != ".PKr" && file.Name() != "PKr-Base.exe" && file.Name() != "PKr-Cli.exe" && file.Name() != "PKr-Base" && file.Name() != "PKr-Cli" {
			if err = os.RemoveAll(path.Join([]string{workspace_path, file.Name()}...)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Create New File of name `dest`.
// Save Data to the File
func SaveDataToFile(data []byte, dest string) error {
	zippedfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer zippedfile.Close()

	zippedfile.Write(data)
	return nil
}

func FolderTree(folder_path string) (map[string]string, error) {
	result := make(map[string]string)

	err := filepath.Walk(folder_path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == ".PKr" || info.Name() == "PKr-Base.exe" || info.Name() == "PKr-Cli.exe" || info.Name() == "PKr-Base" || info.Name() == "PKr-Cli" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		hash, err := encrypt.GenerateHashWithFileIO(f)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(folder_path, path)
		if err != nil {
			return err
		}

		result[relPath] = hash
		return nil
	})

	return result, err
}

func AreUpdatesCached(workspace_path, update_push_range string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(workspace_path, ".PKr", "Files", "Changes"))
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == update_push_range {
			return true, nil
		}
	}
	return false, nil
}

func ClearEmptyDir(root string) error {
	var dirs []string

	// Collect all directories
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Sort in reverse order to ensure children come before parents
	sort.Slice(dirs, func(i, j int) bool {
		return len(strings.Split(dirs[i], string(os.PathSeparator))) >
			len(strings.Split(dirs[j], string(os.PathSeparator)))
	})

	// Now delete empty directories from bottom up
	for _, dir := range dirs {
		empty, err := isDirEmpty(dir)
		if err != nil {
			fmt.Println("Error checking if directory is empty:", err)
			continue
		}
		if empty {
			err = os.Remove(dir)
			if err != nil {
				fmt.Println("Error removing directory:", err)
			}
		}
	}
	return nil
}

// isDirEmpty checks whether a directory is empty
func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	defer f.Close()

	// Read one entry from the directory
	_, err = f.Readdir(1)
	if err == nil {
		return false, nil // Not empty
	}
	if err == io.EOF {
		return true, nil // Empty
	}
	return false, err
}

func UpdateFilesFromWorkspace(workspace_path string, content_path string, changes map[string]string) error {
	for rel_path, change_type := range changes {
		if runtime.GOOS == "windows" {
			rel_path = strings.ReplaceAll(rel_path, "/", "\\")
		} else {
			rel_path = strings.ReplaceAll(rel_path, "\\", "/")
		}

		workspace_file := filepath.Join(workspace_path, rel_path)

		switch change_type {
		case "Removed":
			err := os.Remove(workspace_file)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove %s: %v", workspace_file, err)
			}

			err = ClearEmptyDir(workspace_path)
			if err != nil && !os.IsNotExist(err) {
				fmt.Printf("failed to clear empty dirs in '%s' dir: %v\n", workspace_path, err)
				fmt.Println("Ignorning this Error")
			}

		case "Updated":
			source_file := filepath.Join(content_path, rel_path)

			// Make sure the parent directory exists
			if err := os.MkdirAll(filepath.Dir(workspace_file), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %v", workspace_file, err)
			}

			err := copyFile(source_file, workspace_file)
			if err != nil {
				return fmt.Errorf("failed to update %s: %v", rel_path, err)
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync() // ensure file is fully written
}
