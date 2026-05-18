package model

import "errors"

// Ошибки домена — не зависят от HTTP, базы данных или любой инфраструктуры.
// Сервисный слой возвращает эти ошибки, хендлеры переводят их в HTTP коды.
var (
	// --- Пользователи ---
	ErrEmptyUsername       = errors.New("имя пользователя не может быть пустым")
	ErrUsernameTooLong     = errors.New("имя пользователя не более 50 символов")
	ErrInvalidEmail        = errors.New("некорректный email")
	ErrEmailTooLong        = errors.New("email не более 255 символов")
	ErrPasswordTooShort    = errors.New("пароль не менее 8 символов")
	ErrUserNotFound        = errors.New("пользователь не найден")
	ErrEmailAlreadyExists  = errors.New("email уже занят")
	ErrInvalidCredentials  = errors.New("неверный email или пароль") // намеренно общее — не раскрываем что именно неверно

	// --- JWT / Авторизация ---
	ErrUnauthorized     = errors.New("требуется авторизация")
	ErrForbidden        = errors.New("нет доступа к ресурсу")
	ErrTokenExpired     = errors.New("токен истёк, войдите заново")
	ErrTokenInvalid     = errors.New("невалидный токен")
	ErrTokenRevoked     = errors.New("токен отозван") // logout через blacklist

	// --- Файлы ---
	ErrEmptyFileName       = errors.New("имя файла не может быть пустым")
	ErrFileNameTooLong     = errors.New("имя файла не более 255 символов")
	ErrFileTooLarge        = errors.New("файл превышает максимальный размер 100MB")
	ErrFileNotFound        = errors.New("файл не найден")
	ErrFileAlreadyDeleted  = errors.New("файл уже удалён")
	ErrEncryptionFailed    = errors.New("ошибка шифрования файла")

	// --- Директории ---
	ErrEmptyDirName     = errors.New("имя каталога не может быть пустым")
	ErrDirNameTooLong   = errors.New("имя каталога не более 255 символов")
	ErrInvalidDirName   = errors.New("имя каталога содержит недопустимые символы")
	ErrDirNotFound      = errors.New("каталог не найден")
	ErrDirNotEmpty      = errors.New("каталог не пустой, удалите содержимое сначала")
)
