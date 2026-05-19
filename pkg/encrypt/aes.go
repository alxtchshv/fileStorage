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

// AES реализует AES-256-GCM потоковое шифрование для файлов.
// GCM — аутентифицированное шифрование: защищает и от чтения, и от подделки данных.
// Потоковое: файлы до 100MB обрабатываются чанками, не загружая весь файл в память.
type AES struct {
	key []byte // 32 байта = 256 бит
}

// NewAES создаёт шифратор из hex-строки ключа (64 символа = 32 байта).
// Генерация ключа: openssl rand -hex 32
func NewAES(key string) *AES {
	key = strings.TrimSpace(key)
	decoded, err := hex.DecodeString(key)
	if err != nil {
		log.Fatalf("ENCRYPTION_KEY: неверный hex формат (ожидается 64 hex символа): %v", err)
	}
	if len(decoded) != 32 {
		log.Fatalf("ENCRYPTION_KEY: ожидается 32 байта (64 hex символа), получено %d байт", len(decoded))
	}
	return &AES{key: decoded}
}

// EncryptStream читает plaintext из src, шифрует и пишет в dst.
// Используется при загрузке файла: HTTP body -> encrypt -> MinIO.
// Формат: [12 байт nonce][зашифрованные данные][16 байт authentication tag]
// Nonce генерируется случайно при каждом шифровании (crypto/rand).
func (a *AES) EncryptStream(dst io.Writer, src io.Reader) error {

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// 1. Записать nonce в dst первым (нужен при расшифровке)
	if _, err := dst.Write(nonce); err != nil {
		return err
	}

	cipherBlock, err := aes.NewCipher(a.key)
	if err != nil {
		return err
	}

	// 2. Создать GCM режим
	gcm, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return err
	}

	// 3. Читать src кусками (чанками по 32KB чтобы не загружать весь файл в память)
	buf := make([]byte, 32*1024) // 32KB
	counter := uint64(0)         // для генерации уникального nonce для каждого чанка

	for {
		n, err := src.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 {
			break
		}

		chunk := buf[:n]

		// 4. Шифровать каждый чанк: gcm.Seal(nil, nonce, chunk, nil)
		//    Для потокового GCM нужно использовать counter в nonce для каждого чанка
		chunkNonce := make([]byte, 12)
		copy(chunkNonce, nonce)
		for i := 0; i < 8; i++ {
			chunkNonce[11-i] = byte((counter >> (8 * i)) & 0xff)
		}
		counter++

		ciphertext := gcm.Seal(nil, chunkNonce, chunk, nil)

		// 5. Записывать зашифрованный чанк в dst
		if _, err := dst.Write(ciphertext); err != nil {
			return err
		}
	}

	return nil
}

// DecryptStream читает шифротекст из src, расшифровывает и пишет в dst.
// Используется при скачивании: MinIO -> decrypt -> HTTP response body.
func (a *AES) DecryptStream(dst io.Writer, src io.Reader) error {

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(src, nonce); err != nil {
		return err
	}

	cipherBlock, err := aes.NewCipher(a.key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return err
	}

	buf := make([]byte, 32*1024+16) // 32KB + 16 байт для authentication tag

	for {
		n, err := src.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 {
			break
		}

		ciphertext := buf[:n]

		// 6. Расшифровывать: gcm.Open(nil, nonce, ciphertext, nil)
		chunkNonce := make([]byte, 12)
		copy(chunkNonce, nonce)

		chunkCounter := uint64(0) // для генерации того же nonce, что и при шифровании
		for i := 0; i < 8; i++ {
			chunkNonce[11-i] = byte((chunkCounter >> (8 * i)) & 0xff)
		}
		chunkCounter++

		plaintext, err := gcm.Open(nil, chunkNonce, ciphertext, nil)
		if err != nil {
			return err // данные повреждены или подделаны
		}

		if _, err := dst.Write(plaintext); err != nil {
			return err
		}
	}

	return nil
}

// GenerateKey генерирует случайный 32-байтный ключ.
// Используй при первоначальной настройке: ./server generate-key
func GenerateKey() (string, error) {

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}

	return hex.EncodeToString(key), nil
}
