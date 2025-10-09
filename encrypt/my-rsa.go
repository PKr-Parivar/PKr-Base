package encrypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"path/filepath"

	"os"

	"github.com/PKr-Parivar/PKr-Base/utils"
)

const RSA_KEY_SIZE = 4096

func GenerateRSAKeys() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, RSA_KEY_SIZE)
	if err != nil {
		fmt.Println("Error while Generating RSA Keys:", err)
		fmt.Println("Source: GenerateRSAKeys()")
		return nil, nil
	}
	return privateKey, &privateKey.PublicKey
}

func ParsePrivateKeyToBytes(pkey *rsa.PrivateKey) []byte {
	pkeyBytes := x509.MarshalPKCS1PrivateKey(pkey)
	privatekey_pem_block := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: pkeyBytes,
		},
	)
	return privatekey_pem_block
}

func ParsePublicKeyToBytes(pbkey *rsa.PublicKey) []byte {
	pbkeyBytes := x509.MarshalPKCS1PublicKey(pbkey)
	publickey_pem_block := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pbkeyBytes,
		},
	)
	return publickey_pem_block
}

func StorePrivateKeyInFile(filepath string, pkey *rsa.PrivateKey) error {
	private_pem_key := ParsePrivateKeyToBytes(pkey)
	if private_pem_key == nil {
		return errors.New("could not convert private key to []byte")
	}
	return os.WriteFile(filepath, private_pem_key, 0700)
}

func StorePublicKeyInFile(filepath string, pbkey *rsa.PublicKey) error {
	public_pem_key := ParsePublicKeyToBytes(pbkey)

	if public_pem_key == nil {
		return errors.New("could not convert public key to []byte")
	}
	return os.WriteFile(filepath, public_pem_key, 0700)
}

func RSADecryptData(cipherText string) (string, error) {
	block, _ := pem.Decode([]byte(loadPrivateKey()))
	if block == nil {
		fmt.Println("Error while Parsing, Pem Block is nil")
		fmt.Println("Pls check if the provided Private Key is correct")
		fmt.Println("Source: RSADecryptData()")
		return "", errors.New("error in retrieving the Pem Block")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Println("Error while parsing the Private Key:", err)
		fmt.Println("Source: RSADecryptData()")
		return "", err
	}

	hash := sha256.New()

	baseDecoded, _ := base64.StdEncoding.DecodeString(cipherText)
	label := []byte("")
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privKey, []byte(baseDecoded), label)
	if err != nil {
		fmt.Println("Error while Decrypting Cipher text:", err)
		fmt.Println("Source: RSADecryptData()")
		return "", err
	}
	return string(plaintext), err
}

func RSAEncryptData(data string, publicPemBock string) (string, error) {
	block, _ := pem.Decode([]byte(publicPemBock))
	if block == nil {
		fmt.Println("Error while Parsing, Pem Block is nil")
		fmt.Println("Pls check if the provided Private Key is correct")
		fmt.Println("Source: RSAEncryptData()")
		return "", errors.New("error in retrieving the Pem Block")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		fmt.Println("Error while parsing the Public Key:", err)
		fmt.Println("Source: RSAEncryptData()")
		return "", err
	}

	label := []byte("")
	hash := sha256.New()

	result, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, []byte(data), label)
	if err != nil {
		fmt.Println("Error while Encrypting text:", err)
		fmt.Println("Source: RSAEncryptData()")
		return "", err
	}

	base64Encrypted := base64.StdEncoding.EncodeToString(result)
	return base64Encrypted, nil
}

func GetPublicKey(path string) string {
	key, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error while Reading Public Key:", err)
		fmt.Println("Source: GetPublicKey()")
		return ""
	}
	return string(key)
}

func loadPrivateKey() string {
	my_keys_path, err := utils.GetMyKeysPath()
	if err != nil {
		fmt.Println("Error while Getting Path of My Keys:", err)
		fmt.Println("Source: LoadPrivateKey()")
		return ""
	}
	private_key_path := filepath.Join(my_keys_path, "private.pem")
	key, err := os.ReadFile(private_key_path)
	if err != nil {
		fmt.Println("Error in Loading Private Key:", err)
		fmt.Println("Source: LoadPrivateKey()")
		return ""
	}
	return string(key)
}
