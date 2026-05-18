package encrypt

import (
	"io"
)

// AES реализует AES-256-GCM потоковое шифрование для файлов.
// Почему GCM? Это аутентифицированное шифрование — защищает от модификации зашифрованных данных.
// Почему потоковое? Файлы могут быть большими (до 100MB), нельзя загружать весь файл в память.
type AES struct {
	key []byte // 32 байта = 256 бит
}

// NewAES создаёт шифратор. Ключ должен быть ровно 32 байта.
// Если ключ задан как hex-строка в конфиге — декодировать через hex.DecodeString.
func NewAES(key string) *AES {
	// 1. Декодировать key из hex строки в []byte
	// 2. Проверить что длина ключа == 32 байта (panic при нарушении — это конфиг ошибка)
	// 3. Вернуть &AES{key: keyBytes}
	return &AES{key: []byte(key)} // упрощённая заглушка
}

// EncryptStream читает plaintext из src, шифрует и пишет в dst.
// Используется при загрузке файла: HTTP body -> encrypt -> MinIO.
// Формат: [12 байт nonce][зашифрованные данные][16 байт authentication tag]
// Nonce генерируется случайно при каждом шифровании (crypto/rand).
func (a *AES) EncryptStream(dst io.Writer, src io.Reader) error {
	// 1. Сгенерировать nonce: make([]byte, 12); io.ReadFull(rand.Reader, nonce)
	// 2. Записать nonce в dst первым (нужен при расшифровке)
	// 3. Создать aes.NewCipher(a.key) -> cipher.NewGCM(block)
	// 4. Читать src кусками (чанками по 32KB чтобы не загружать весь файл в память)
	// 5. Шифровать каждый чанк: gcm.Seal(nil, nonce, chunk, nil)
	//    Внимание: для потокового GCM нужно использовать counter в nonce для каждого чанка
	//    или использовать специализированную библиотеку (golang.org/x/crypto/chacha20poly1305)
	// 6. Записывать зашифрованный чанк в dst
	return nil
}

// DecryptStream читает шифротекст из src, расшифровывает и пишет в dst.
// Используется при скачивании: MinIO -> decrypt -> HTTP response body.
func (a *AES) DecryptStream(dst io.Writer, src io.Reader) error {
	// 1. Прочитать первые 12 байт — это nonce
	// 2. Создать cipher аналогично EncryptStream
	// 3. Читать src кусками (chunk + 16 байт тег)
	// 4. Расшифровывать: gcm.Open(nil, nonce, encryptedChunk, nil)
	//    Если authentication tag невалидный — вернуть ошибку (данные повреждены/подделаны)
	// 5. Писать расшифрованные данные в dst
	return nil
}

// GenerateKey генерирует случайный 32-байтный ключ.
// Используй при первоначальной настройке: ./server generate-key
func GenerateKey() (string, error) {
	// 1. key := make([]byte, 32)
	// 2. io.ReadFull(rand.Reader, key)
	// 3. Вернуть hex.EncodeToString(key)
	return "", nil
}
