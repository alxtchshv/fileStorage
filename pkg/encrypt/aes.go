package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"strings"
)

const chunkSize = 32 * 1024 // 32 KB

// AES реализует AES-256-GCM потоковое шифрование для файлов.
// GCM — аутентифицированное шифрование: защищает и от чтения, и от подделки данных.
// Потоковое: файлы обрабатываются чанками по 32KB, весь файл не загружается в память.
type AES struct {
	key []byte // 32 байта = 256 бит
}

// NewAES создаёт шифратор из hex-строки ключа (64 символа = 32 байта).
// Генерация: openssl rand -hex 32
func NewAES(key string) *AES {
	key = strings.TrimSpace(key)
	decoded, err := hex.DecodeString(key)
	if err != nil {
		log.Fatalf("ENCRYPTION_KEY: неверный hex формат: %v", err)
	}
	if len(decoded) != 32 {
		log.Fatalf("ENCRYPTION_KEY: нужно 32 байта (64 hex символа), получено %d", len(decoded))
	}
	return &AES{key: decoded}
}

// EncryptStream читает plaintext из src, шифрует чанками и пишет в dst.
// Формат на диске: [12 байт nonce][чанк1: plaintext+16байт tag][чанк2]...
// Nonce один на весь файл, для каждого чанка формируется chunkNonce = nonce XOR counter.
func (a *AES) EncryptStream(dst io.Writer, src io.Reader) error {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	if _, err := dst.Write(nonce); err != nil {
		return err
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	buf := make([]byte, chunkSize)
	counter := uint64(0) // счётчик чанков — вне цикла

	for {
		n, readErr := io.ReadFull(src, buf)
		if n > 0 {
			ciphertext := gcm.Seal(nil, chunkNonce(nonce, counter), buf[:n], nil)
			counter++
			if _, err := dst.Write(ciphertext); err != nil {
				return err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// DecryptStream читает шифротекст из src, расшифровывает чанками и пишет в dst.
// Каждый зашифрованный чанк = chunkSize + gcm.Overhead() (16 байт тег).
func (a *AES) DecryptStream(dst io.Writer, src io.Reader) error {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(src, nonce); err != nil {
		return err
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	encChunkSize := chunkSize + gcm.Overhead() // 32768 + 16 = 32784
	buf := make([]byte, encChunkSize)
	counter := uint64(0) // счётчик чанков — вне цикла

	for {
		// io.ReadFull читает ровно len(buf) байт.
		// Частичное чтение (например при стриминге из MinIO) не сломает чанки.
		n, readErr := io.ReadFull(src, buf)
		if n > 0 {
			plaintext, err := gcm.Open(nil, chunkNonce(nonce, counter), buf[:n], nil)
			if err != nil {
				return err // данные повреждены или подделаны
			}
			counter++
			if _, err := dst.Write(plaintext); err != nil {
				return err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// chunkNonce формирует уникальный nonce для каждого чанка.
// Берёт base nonce и XOR-ит последние 8 байт со счётчиком.
func chunkNonce(base []byte, counter uint64) []byte {
	n := make([]byte, 12)
	copy(n, base)
	for i := 0; i < 8; i++ {
		n[11-i] = base[11-i] ^ byte(counter>>(8*i))
	}
	return n
}

// GenerateKey генерирует случайный 32-байтный ключ в hex.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
