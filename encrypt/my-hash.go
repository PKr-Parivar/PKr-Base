package encrypt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"sort"
	"sync"
)

var bufferPool = sync.Pool{
	// var buffer = make([]byte, 32*1024) ;; cpu2.prof
	// var buffer = make([]byte, 128*1024) ;; cpu3.prof --> Most Efficient 
	// 										( from personal limited Testing -- Increase TestCases in Future )
	// var buffer = make([]byte, 256*1024) ;; cpu4.prof

	// 30% increase over original -- it seems I cannot reduce the time take to `open` file
	New: func() any {
		return make([]byte, 128*1024)
	},
}

var hashPool = sync.Pool{
	New: func() any {
		return sha256.New()
	},
}

var hexPool = sync.Pool{
	New: func() any {
		return make([]byte, 64) // fixed length for hex-encoded SHA-256
	},
}

func GenerateHashFromFileNames_BufferedAndPooled(file_path string) (string, error) {
	f, err := os.Open(file_path)
	if err != nil {
		fmt.Println("Error while Generating Hash with File Path:", err)
		fmt.Println("Source: GenerateHashFromFileNames_BufferedAndPooled()")
		return "", err
	}
	defer f.Close()

	h := hashPool.Get().(hash.Hash)
	h.Reset()

	buf := bufferPool.Get().([]byte)

	// TODO: Find a way to compute hash in chunks -- Compare chunk (h.sum(nil)) with (h.sum(chunk))
	// According to gpt -- not possible idk why ???
	// The buffer at the end of the day loads the entire file to h -- so more GC
	// See if extra Performance is really needed
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		hashPool.Put(h)
		bufferPool.Put(buf)
		fmt.Println("Error while Copying from file object:", err)
		fmt.Println("Source: GenerateHashFromFileNames_BufferedAndPooled()")
		return "", err
	}

	hashBytes := h.Sum(nil)

	h.Reset() // important: reuse in clean state
	hashPool.Put(h)
	bufferPool.Put(buf)

	hashDst := hexPool.Get().([]byte)

	hex.Encode(hashDst, hashBytes)
	out := string(hashDst)

	hexPool.Put(hashDst)
	return out, nil
}

func GenerateHashWithFilePath(file_path string) (string, error) {
	data, err := os.ReadFile(file_path)
	if err != nil {
		fmt.Println("Error while Generating Hash with File Path:", err)
		fmt.Println("Source: GenerateHashWithFilePath()")
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func GenerateHashWithFileIO(file *os.File) (string, error) {
	_, err := file.Seek(0, 0)
	if err != nil {
		fmt.Println("Error while Seeking file:", err)
		fmt.Println("Source: GenerateHashWithFileIO()")
		return "", err
	}

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		fmt.Println("Error while Copying from file object:", err)
		fmt.Println("Source: GenerateHashWithFileIO()")
		return "", err
	}

	hash := h.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}

// Generates Hash using Entire FileName and its Path
func GeneratHashFromFileNames(files_hash_list []string) string {
	sort.Strings(files_hash_list) // Step 1: sort for deterministic result

	combined := ""
	for _, h := range files_hash_list {
		combined += h
	}

	// Step 3: hash the combined string
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}
