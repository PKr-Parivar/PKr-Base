package encrypt

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

const DATA_CHUNK = 1024                        // 1KB
const FLUSH_AFTER_EVERY_X_MB = 5 * 1024 * 1024 // 5 MB

func AESGenerakeKey(length int) ([]byte, error) {
	// keep length 16, 24, 32 -> 128, 192, 256 respectively
	key := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func AESGenerateIV() ([]byte, error) {
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	return iv, nil
}

// Same func for Encrypt & Decrypt
func EncryptDecryptChunk(data, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("Error while Creating New Cipher Block:", err)
		fmt.Println("Source: EncryptDecryptChunk()")
		return nil, err
	}

	encrypted_decrypted := make([]byte, len(data))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(encrypted_decrypted, data)

	return encrypted_decrypted, nil
}

func EncryptZipFileAndStore(zipped_filepath, zip_enc_path string, key, iv []byte) error {
	zipped_filepath_obj, err := os.Open(zipped_filepath)
	if err != nil {
		fmt.Println("Failed to Open Zipped File:", err)
		fmt.Println("Source: EncryptZipFileAndStore()")
		return err
	}
	defer zipped_filepath_obj.Close()

	zip_enc_file_obj, err := os.Create(zip_enc_path)
	if err != nil {
		fmt.Println("Failed to Create & Open Enc Zipped File:", err)
		fmt.Println("Source: EncryptZipFileAndStore()")
		return err
	}
	defer zip_enc_file_obj.Close()

	buffer := make([]byte, DATA_CHUNK)
	reader := bufio.NewReader(zipped_filepath_obj)
	writer := bufio.NewWriter(zip_enc_file_obj)

	// Reading from Zip File, Encrypting it & Writing it to Enc Zip File
	offset := 0
	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error while Reading Zip File:", err)
			fmt.Println("Source: EncryptZipFileAndStore()")
			return err
		}
		encrypted, err := EncryptDecryptChunk(buffer[:n], key, iv)
		if err != nil {
			fmt.Println("Failed to Encrypt Chunk:", err)
			fmt.Println("Source: EncryptZipFileAndStore()")
			return err
		}

		_, err = writer.Write(encrypted)
		if err != nil {
			fmt.Println("Failed to Write Chunk to File:", err)
			fmt.Println("Source: EncryptZipFileAndStore()")
			return err
		}

		// Flush buffer to disk after 'FLUSH_AFTER_EVERY_X_CHUNK'
		if offset%FLUSH_AFTER_EVERY_X_MB == 0 {
			err = writer.Flush()
			if err != nil {
				fmt.Println("Error flushing 'writer' after X KB/MB buffer:", err)
				fmt.Println("Soure: EncryptZipFileAndStore()")
				return err
			}
		}
		offset += n
	}

	// Flush buffer to disk at end
	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing 'writer' buffer:", err)
		fmt.Println("Soure: EncryptZipFileAndStore()")
		return err
	}
	zipped_filepath_obj.Close() // Close Obj now, so we can delete zip file
	zip_enc_file_obj.Close()

	// Removing Zip File
	err = os.Remove(zipped_filepath)
	if err != nil {
		fmt.Println("Error deleting zip file:", err)
		fmt.Println("Source: EncryptZipFileAndStore()")
		return err
	}
	return nil
}
