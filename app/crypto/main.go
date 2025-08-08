package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"hash/fnv"
	"strconv"
)

func HashString(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return strconv.FormatUint(h.Sum64(), 10)
}

func Encrypt(v string, k string) (string, error) {
	value := []byte(v)
	key := []byte(k)

	md5 := md5.Sum(key)
	key = md5[:]

	value = pad(value)

	iv := random(aes.BlockSize)

	block, err := aes.NewCipher(key)

	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, len(value))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, value)

	buf := bytes.NewBuffer(iv)
	buf.Write(ciphertext)
	result := buf.Bytes()

	return base64.StdEncoding.EncodeToString(result), nil
}

func Decrypt(v string, k string) (string, error) {
	value, err := base64.StdEncoding.DecodeString(v)

	if err != nil {
		return "", err
	}

	key := []byte(k)

	md5 := md5.Sum(key)
	key = md5[:]

	iv := value[:aes.BlockSize]

	ciphertext := value[aes.BlockSize:]

	block, err := aes.NewCipher(key)

	if err != nil {
		return "", err
	}

	text := make([]byte, len(ciphertext))

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(text, ciphertext)

	return string(unpad(text)), nil
}

func pad(value []byte) []byte {
	pdd := aes.BlockSize - (len(value) % aes.BlockSize)
	buf := bytes.NewBuffer(value)
	buf.Write(bytes.Repeat([]byte{byte(pdd)}, pdd))
	return buf.Bytes()
}

func unpad(value []byte) []byte {
	length := len(value)
	pdd := value[length-1:]
	before := length - int(pdd[0])

	if bytes.Equal(value[before:], bytes.Repeat(pdd, int(pdd[0]))) {
		return value[:before]
	}

	return value
}

func random(size int) []byte {
	r := make([]byte, size)
	rand.Read(r)
	return r
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789?_!-"
	result := make([]byte, length)
	charsetLength := len(charset)

	randomBytes := random(length)
	for i := range randomBytes {
		result[i] = charset[int(randomBytes[i])%charsetLength]
	}

	return string(result)
}
