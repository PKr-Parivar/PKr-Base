package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/PKr-Parivar/PKr-Base/encrypt"
)

var TREE_REL_PATH = filepath.Join(".PKr", "file-tree.json")

type FileTree struct {
	Nodes []Node
}

type Node struct {
	FilePath string `json:"file_path"`
	Hash     string `json:"hash"`
}

type FilePath struct {
	FilePath    string
	RelFilePath string
}

func CreateFileTreeIfNotExits(workspace_path string) error {
	tree_file_path := filepath.Join(workspace_path, TREE_REL_PATH)
	if _, err := os.Stat(tree_file_path); os.IsExist(err) {
		fmt.Println("File Tree Already Exists")
		return nil
	}

	fileTree, err := GetNewTree(workspace_path)
	if err != nil {
		fmt.Println("Error while Getting New Tree: ", err)
		fmt.Println("Source: CreateFileTreeIfNotExits()")
		return err
	}

	err = WriteToFileTree(workspace_path, fileTree)
	if err != nil {
		fmt.Println("Error while Writing in File Tree: ", err)
		fmt.Println("Source: CreateFileTreeIfNotExits()")
		return err
	}
	return nil
}

func FetchAllFilesPaths(folder_path string) ([]FilePath, error) {
	files := []FilePath{}

	err := filepath.Walk(folder_path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // skip files we can't read
		}

		if info.IsDir() && (info.Name() == ".PKr" || info.Name() == "tmp") {
			return filepath.SkipDir
		} else if !info.IsDir() {
			if info.Name() == "PKr-Base.exe" || info.Name() == "PKr-Cli.exe" || info.Name() == "PKr-Base" || info.Name() == "PKr-Cli" {
				return nil
			}

			relPath, err := filepath.Rel(folder_path, path)
			if err != nil {
				fmt.Println("Error while Getting Relative Path:", err)
				fmt.Println("Source: FetchAllFilesPaths()")
				return err
			}

			files = append(files, FilePath{
				FilePath:    path,
				RelFilePath: relPath,
			})
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking the path:", err)
		fmt.Println("Source: FetchAllFilesPaths()")
		return []FilePath{}, err
	}

	return files, nil

}

// Fetchs all Files and Than go routines the hash computations.
// Reason for change -- to switch and process other files while 
// waiting for Syscalls. 
// From personal Testing -------------
// 		- Total Files: 2300
// 		- Total Size: 110MB
// 		- From 40 Sec -> 2.42 Sec (num of cpu cores - 16)
func GetNewTree(workspace_path string) (FileTree, error) {
	file_paths, err := FetchAllFilesPaths(workspace_path)
	if err != nil {
		fmt.Println("Error while Getting all File Paths from the Folder: ", err)
		fmt.Println("Source: GetNewTree()")
		return FileTree{}, err
	}

	n_files := len(file_paths)
	numWorkers := min(runtime.NumCPU() * 2, n_files)

	partitionSize := (n_files + numWorkers - 1) / numWorkers // `(n_files + numWorkers - 1)` : Run Atleast once
	nodes := make([]Node, n_files)
	var wg sync.WaitGroup

	// No i:= 0 is ok -- Please Dont make it into range ; Again Cannot Read that Shit
	for i := 0; i < numWorkers; i++ {
		start := i * partitionSize
		end := (i + 1) * partitionSize

		// Do Not Min() this -- I cannot Read or Understand That Shit
		if end > n_files {
			end = n_files
		}
		wg.Add(1)
		go func(jobs []FilePath, res []Node) {
			for j, job := range jobs {

				hash, err := encrypt.GenerateHashFromFileNames_BufferedAndPooled(job.FilePath)
				if err == nil { // I know ; and yes keep it this way -- please dont change
					res[j] = Node{FilePath: job.RelFilePath, Hash: hash}
				} else {
					// Needs Proper Error Handling - Something without channels (impacts performance)
					fmt.Println("Error while Generating Hash for files: ", err)
					fmt.Println("Source: GetNewTree()")
					continue
				}
			}
			wg.Done()
		}(file_paths[start:end], nodes[start:end])
	}
	wg.Wait()

	return FileTree{
		Nodes: nodes,
	}, nil

}

func ReadFromTreeFile(workspace_tree_path string) (FileTree, error) {
	file, err := os.Open(filepath.Join(workspace_tree_path, TREE_REL_PATH))
	if err != nil {
		fmt.Println("Error while opening tree file:", err)
		fmt.Println("Source: ReadFromTreeFile()")
		return FileTree{}, err
	}
	defer file.Close()

	var fileTree FileTree
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&fileTree)
	if err != nil {
		fmt.Println("Error while Decoding JSON Data from tree file:", err)
		fmt.Println("Source: ReadFromTreeFile()")
		return FileTree{}, err
	}
	return fileTree, nil
}

func WriteToFileTree(workspace_tree_path string, FileTree FileTree) error {
	jsonData, err := json.MarshalIndent(FileTree, "", "	")
	if err != nil {
		fmt.Println("Error while Marshalling the file-tree to JSON:", err)
		fmt.Println("Source: WriteToFileTree()")
		return err
	}

	err = os.WriteFile(filepath.Join(workspace_tree_path, TREE_REL_PATH), jsonData, 0700)
	if err != nil {
		fmt.Println("Error while writing data in file-tree:", err)
		fmt.Println("Source: WriteToFileTree()")
		return err
	}
	return nil
}

func CompareTrees(oldTree, newTree FileTree) []FileChange {
	// Build lookup maps
	oldMap := make(map[string]string)
	newMap := make(map[string]string)

	for _, node := range oldTree.Nodes {
		oldMap[node.FilePath] = node.Hash
	}

	for _, node := range newTree.Nodes {
		newMap[node.FilePath] = node.Hash
	}

	var changes []FileChange

	// Detect created or updated
	for path, newHash := range newMap {
		oldHash, exists := oldMap[path]
		if !exists {
			// New file
			changes = append(changes, FileChange{
				FilePath: path,
				FileHash: newHash,
				Type:     "Updated",
			})
		} else if newHash != oldHash {
			// Updated file
			changes = append(changes, FileChange{
				FilePath: path,
				FileHash: newHash,
				Type:     "Updated",
			})
		}
	}

	// Detect removed
	for path, oldHash := range oldMap {
		if _, exists := newMap[path]; !exists {
			changes = append(changes, FileChange{
				FilePath: path,
				FileHash: oldHash,
				Type:     "Removed",
			})
		}
	}
	return changes
}
